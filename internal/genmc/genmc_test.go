package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The checked-in files must be exactly what the generator writes.
func TestGeneratedFilesUpToDate(t *testing.T) {
	asm, tab := generate()
	for name, want := range map[string]string{"luma_arm64.s": asm, "luma_arm64_tab.go": tab} {
		got, err := os.ReadFile(filepath.Join("..", "..", "mc", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s is stale: run go generate ./mc", name)
		}
	}
}

func TestWriteAndExit(t *testing.T) {
	dir := t.TempDir()
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = []string{"genmc", dir}
	main()
	for _, name := range []string{"luma_arm64.s", "luma_arm64_tab.go"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Error(err)
		}
	}

	// The default directory does not exist from the package directory: the
	// write fails and the program exits 1.
	code := 0
	savedExit := osExit
	defer func() { osExit = savedExit }()
	osExit = func(c int) { code = c }
	os.Args = []string{"genmc"}
	main()
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
}
