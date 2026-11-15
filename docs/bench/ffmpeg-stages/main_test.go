package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeRunner answers like ffmpeg -benchmark: the decode time grows with the
// number of P pictures (the size of the file stands for it) at a rate that
// depends on the case, so the report's differences are known.
func fakeRunner(perMB map[string]float64, dir string) runner {
	return func(args []string) (string, error) {
		file := args[len(args)-4]
		if strings.HasPrefix(args[0], "taskset") {
			file = args[len(args)-4]
		}
		for name, v := range perMB {
			if strings.Contains(file, name) {
				frames := 100.0
				if strings.HasSuffix(file, "-300.264") {
					frames = 300
				}
				return "bench: utime=" + trim(0.01+frames*v) + "s stime=0.000s rtime=1s\n", nil
			}
		}
		return "", errors.New("unknown case " + file)
	}
}

func trim(v float64) string { return strconv.FormatFloat(v, 'f', 9, 64) }

func TestDecodeTime(t *testing.T) {
	ok := func(args []string) (string, error) { return "x\nbench: utime=1.5s stime=0.25s rtime=2s\n", nil }
	if v, err := decodeTime(ok, "ffmpeg", -1, "f.264"); err != nil || v != 1.75 {
		t.Errorf("got %v, %v", v, err)
	}
	var seen []string
	pin := func(args []string) (string, error) { seen = args; return "bench: utime=1s stime=0s", nil }
	if _, err := decodeTime(pin, "ffmpeg", 7, "f.264"); err != nil || seen[0] != "taskset" || seen[2] != "7" {
		t.Errorf("pinned args %v, %v", seen, err)
	}
	if _, err := decodeTime(func([]string) (string, error) { return "nothing", nil }, "ffmpeg", -1, "f"); err == nil {
		t.Error("no bench line: no error")
	}
	if _, err := decodeTime(func([]string) (string, error) { return "boom", errors.New("exit 1") }, "ffmpeg", -1, "f"); err == nil {
		t.Error("runner error not reported")
	}
}

func TestMedian(t *testing.T) {
	if median([]float64{3, 1, 2}) != 2 || median([]float64{4, 1, 3, 2}) != 2.5 {
		t.Error("median")
	}
}

func TestExecReport(t *testing.T) {
	dir := t.TempDir()
	load := filepath.Join(dir, "loadavg")
	if err := os.WriteFile(load, []byte("0.10 0.20 0.30 1/2 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Per macroblock per picture, in seconds: G 100 ns, luma j 140 ns, chroma xy
	// 120 ns, chroma x 110 ns, chroma y 110 ns, weight 130 ns. The 1x1 picture
	// has one macroblock.
	perMB := map[string]float64{}
	cs := cases(1, 1)
	rate := map[string]float64{"luma G": 100e-9, "luma j": 140e-9, "chroma xy": 120e-9, "chroma x": 110e-9, "chroma y": 110e-9}
	for i, c := range cs {
		r, ok := rate[c.name]
		if !ok {
			r = 105e-9
			if strings.HasPrefix(c.name, "weight") {
				r = 130e-9
			}
		}
		perMB[fmt.Sprintf("case%02d-", i)] = r
	}
	o := &options{
		ffmpeg: "ffmpeg", cpu: -1, dir: filepath.Join(dir, "streams"), rounds: 3, f1: 100, f2: 300, wMB: 1, hMB: 1,
		maxLoad: 1, loadavg: load, run: fakeRunner(perMB, dir),
	}
	var out bytes.Buffer
	o.out = &out
	if err := o.exec(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"luma j", "40.0", "weight uni 16x16", "30.0"} {
		if !strings.Contains(s, want) {
			t.Errorf("report lacks %q:\n%s", want, s)
		}
	}
	// luma j: 140 - 100 = 40 over G, minus the 2-D chroma interpolation (120 - 100).
	if !strings.Contains(s, "luma j") || !strings.Contains(lineOf(s, "luma j"), "20.0") {
		t.Errorf("luma-only figure of j is not 20.0:\n%s", s)
	}

	// Load above the limit is refused; an unreadable loadavg is ignored; a
	// directory that cannot be made, and a failing runner, are reported.
	if err := os.WriteFile(load, []byte("9.00 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := o.exec(); err == nil || !strings.Contains(err.Error(), "load") {
		t.Errorf("load not refused: %v", err)
	}
	o.loadavg = filepath.Join(dir, "missing")
	o.dir = filepath.Join(load, "x") // a file in the path
	if err := o.exec(); err == nil {
		t.Error("unusable dir accepted")
	}
	o.dir = filepath.Join(dir, "s2")
	o.run = func([]string) (string, error) { return "", errors.New("no ffmpeg") }
	if err := o.exec(); err == nil {
		t.Error("runner error accepted")
	}
	// Unwritable stream file.
	o.run = fakeRunner(perMB, dir)
	if err := os.Remove(filepath.Join(o.dir, "case00-100.264")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(o.dir, "case00-100.264"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := o.exec(); err == nil {
		t.Error("stream write error accepted")
	}
}

func lineOf(s, key string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, key) {
			return l
		}
	}
	return ""
}

func TestRootCmdAndMain(t *testing.T) {
	cmd := RootCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"--ffmpeg", "/nonexistent/ffmpeg", "--dir", t.TempDir(), "--max-load", "1e9", "--rounds", "1", "--width-mbs", "1", "--height-mbs", "1"})
	if err := cmd.Execute(); err == nil {
		t.Error("a missing ffmpeg was accepted")
	}
	code := 0
	saved := osExit
	defer func() { osExit = saved }()
	osExit = func(c int) { code = c }
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	os.Args = []string{"ffmpegstages", "--ffmpeg", "/nonexistent/ffmpeg", "--dir", t.TempDir(), "--max-load", "1e9", "--rounds", "1", "--width-mbs", "1", "--height-mbs", "1"}
	main()
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
}
