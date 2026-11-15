package main

import (
	"os"
	"path/filepath"
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

func TestWriteAndExit(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "lf"), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = []string{"genlf86", root}
	main()
	for name := range files() {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Error(err)
		}
	}

	// The default root, "." (this package's directory), has no lf directory:
	// the write fails and the program exits 1.
	code := 0
	savedExit := osExit
	defer func() { osExit = savedExit }()
	osExit = func(c int) { code = c }
	os.Args = []string{"genlf86"}
	main()
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
}
