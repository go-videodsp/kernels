// Command ffmpegstages times FFmpeg's H.264 decoder on synthetic streams that
// isolate one stage, so that a kernel of go-videodsp/kernels can be compared
// with what FFmpeg spends on the same stage on the same machine.
//
// The streams (stream.go) hold one IDR picture and N P pictures of
// P_L0_16x16 macroblocks with a single motion vector, no residual and no
// deblocking. The decode time of N1 and N2 P pictures is measured, interleaved
// over several rounds, with ffmpeg -benchmark -threads 1; the difference, per
// picture and per macroblock, removes the IDR picture, the start-up and the
// per-macroblock parsing that every case shares. What remains for case X
// minus case G (integer vector, no weights) is the stage under test.
//
//   - position cases: the sixteen quarter-sample luma positions of Table 8-12.
//     The chroma vector is the luma vector, so a fractional luma position also
//     runs FFmpeg's chroma interpolation; three more cases (an integer luma
//     vector of one sample, which is a half-sample chroma vector) measure the
//     chroma interpolation alone, and it is subtracted to give the luma-only
//     figure.
//   - weight case: explicit luma weighted prediction of 8.4.2.3.2 on the
//     integer vector (chroma weights are not coded, so FFmpeg weights only
//     luma).
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var osExit = os.Exit

// letters is Table 8-12 indexed xFrac + 4*yFrac.
var letters = [16]string{"G", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "n", "p", "q", "r"}

// benchCase is one stream.
type benchCase struct {
	name string
	p    params
}

func cases(wMB, hMB int) []benchCase {
	var cs []benchCase
	for i, l := range letters {
		cs = append(cs, benchCase{"luma " + l, params{wMB: wMB, hMB: hMB, mvx: i & 3, mvy: i >> 2}})
	}
	// An integer luma vector of one sample is a half-sample chroma vector.
	cs = append(cs,
		benchCase{"chroma xy", params{wMB: wMB, hMB: hMB, mvx: 4, mvy: 4}},
		benchCase{"chroma x", params{wMB: wMB, hMB: hMB, mvx: 4, mvy: 0}},
		benchCase{"chroma y", params{wMB: wMB, hMB: hMB, mvx: 0, mvy: 4}},
		benchCase{"weight uni 16x16 (denom 5, w 40, o 3)", params{wMB: wMB, hMB: hMB, weight: &weights{5, 40, 3}}})
	return cs
}

type runner func(args []string) (string, error)

func execRunner(args []string) (string, error) {
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	return string(out), err
}

var benchLine = regexp.MustCompile(`utime=([0-9.]+)s stime=([0-9.]+)s`)

// decodeTime runs one decode and returns its CPU time (user + system) in
// seconds.
func decodeTime(run runner, ffmpeg string, cpu int, file string) (float64, error) {
	args := []string{ffmpeg, "-hide_banner", "-benchmark", "-threads", "1", "-i", file, "-f", "null", "-"}
	if cpu >= 0 {
		args = append([]string{"taskset", "-c", strconv.Itoa(cpu)}, args...)
	}
	out, err := run(args)
	if err != nil {
		return 0, fmt.Errorf("%s: %w: %s", file, err, out)
	}
	m := benchLine.FindStringSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("%s: no bench line in %q", file, out)
	}
	u, _ := strconv.ParseFloat(m[1], 64)
	s, _ := strconv.ParseFloat(m[2], 64)
	return u + s, nil
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

type options struct {
	ffmpeg   string
	cpu      int
	dir      string
	rounds   int
	f1, f2   int
	wMB, hMB int
	maxLoad  float64
	loadavg  string
	run      runner
	out      io.Writer
}

// measure returns the median over the rounds of the per-macroblock time of
// each case in nanoseconds.
func (o *options) measure(cs []benchCase) (map[string]float64, error) {
	files := make(map[string][2]string)
	for i, c := range cs {
		var f [2]string
		for k, n := range []int{o.f1, o.f2} {
			p := c.p
			p.frames = n
			f[k] = filepath.Join(o.dir, fmt.Sprintf("case%02d-%d.264", i, n))
			if err := os.WriteFile(f[k], stream(p), 0o644); err != nil {
				return nil, err
			}
		}
		files[c.name] = f
	}
	samples := make(map[string][]float64)
	for r := 0; r < o.rounds; r++ {
		for _, c := range cs {
			var t [2]float64
			for k := range t {
				v, err := decodeTime(o.run, o.ffmpeg, o.cpu, files[c.name][k])
				if err != nil {
					return nil, err
				}
				t[k] = v
			}
			perFrame := (t[1] - t[0]) / float64(o.f2-o.f1)
			samples[c.name] = append(samples[c.name], perFrame*1e9/float64(o.wMB*o.hMB))
		}
	}
	res := make(map[string]float64)
	for name, v := range samples {
		res[name] = median(v)
	}
	return res, nil
}

// report prints the table: the time per macroblock of each case, the excess
// over the integer-vector case, and for the luma positions the excess without
// the chroma interpolation the same vector triggers.
func (o *options) report(cs []benchCase, ns map[string]float64) {
	base := ns["luma G"]
	chroma := func(fx, fy int) float64 {
		switch {
		case fx != 0 && fy != 0:
			return ns["chroma xy"] - base
		case fx != 0:
			return ns["chroma x"] - base
		case fy != 0:
			return ns["chroma y"] - base
		}
		return 0
	}
	fmt.Fprintf(o.out, "%-40s %10s %10s %10s\n", "case", "ns/MB", "- G", "luma only")
	for i, c := range cs {
		if i < 16 {
			fmt.Fprintf(o.out, "%-40s %10.1f %10.1f %10.1f\n", c.name, ns[c.name], ns[c.name]-base, ns[c.name]-base-chroma(i&3, i>>2))
			continue
		}
		fmt.Fprintf(o.out, "%-40s %10.1f %10.1f %10s\n", c.name, ns[c.name], ns[c.name]-base, "")
	}
}

func (o *options) exec() error {
	if b, err := os.ReadFile(o.loadavg); err == nil {
		f := strings.Fields(string(b))
		if v, err := strconv.ParseFloat(f[0], 64); err == nil && v > o.maxLoad {
			return fmt.Errorf("1-minute load %.2f is above %.2f: another session is using this host, wait", v, o.maxLoad)
		}
	}
	if err := os.MkdirAll(o.dir, 0o755); err != nil {
		return err
	}
	cs := cases(o.wMB, o.hMB)
	ns, err := o.measure(cs)
	if err != nil {
		return err
	}
	o.report(cs, ns)
	return nil
}

// RootCmd builds the command.
func RootCmd() *cobra.Command {
	o := &options{run: execRunner, loadavg: "/proc/loadavg"}
	cmd := &cobra.Command{
		Use:           "ffmpegstages",
		Short:         "time FFmpeg's H.264 decoder on synthetic single-stage streams",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.out = cmd.OutOrStdout()
			return o.exec()
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.ffmpeg, "ffmpeg", "ffmpeg", "the ffmpeg binary")
	f.IntVar(&o.cpu, "cpu", -1, "pin each decode to this CPU with taskset (-1: no pinning)")
	f.StringVar(&o.dir, "dir", "streams", "directory for the generated streams")
	f.IntVar(&o.rounds, "rounds", 5, "interleaved rounds (the median is reported)")
	f.IntVar(&o.f1, "frames1", 100, "P pictures of the short stream")
	f.IntVar(&o.f2, "frames2", 300, "P pictures of the long stream")
	f.IntVar(&o.wMB, "width-mbs", 120, "picture width in macroblocks")
	f.IntVar(&o.hMB, "height-mbs", 68, "picture height in macroblocks")
	f.Float64Var(&o.maxLoad, "max-load", 1.0, "refuse to run when the 1-minute load is above this")
	return cmd
}

func main() {
	cmd := RootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ffmpegstages:", err)
		osExit(1)
	}
}
