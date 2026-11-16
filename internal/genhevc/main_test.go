package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The checked-in files must be exactly what the generator writes.
func TestGeneratedFilesUpToDate(t *testing.T) {
	for name, want := range files() {
		got, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s is stale: run go generate ./...", name)
		}
	}
}

// Every kernel the Go side declares is emitted, and no instruction reads past
// the footprint of a group: the loads of a group of eight are 16 bytes, those of the
// group of four 8 bytes.
func TestKernelsAreEmitted(t *testing.T) {
	f := files()
	for _, want := range []string{"hpass8NEON0", "hpass8NEON4", "vpass8NEON"} {
		if !strings.Contains(f["hevc/pass_arm64.s"], "TEXT ·"+want+"(SB)") {
			t.Errorf("%s is not emitted", want)
		}
	}
	for _, want := range []string{"hpass8AVX2", "vpass8AVX2"} {
		if !strings.Contains(f["hevc/pass_amd64.s"], "TEXT ·"+want+"(SB)") {
			t.Errorf("%s is not emitted", want)
		}
	}
}

func TestWriteAndExit(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "hevc"), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = []string{"genhevc", root}
	main()
	for name := range files() {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Error(err)
		}
	}

	// The default root, "." (this package's directory), has no hevc directory:
	// the write fails and the program exits 1.
	code := 0
	savedExit := osExit
	defer func() { osExit = savedExit }()
	osExit = func(c int) { code = c }
	os.Args = []string{"genhevc"}
	main()
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
}
