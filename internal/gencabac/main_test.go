package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The checked-in files must be exactly what the generator writes.
func TestGeneratedFilesUpToDate(t *testing.T) {
	for name, want := range map[string]string{
		"decode_amd64.s": amd64(),
		"decode_arm64.s": arm64(),
	} {
		got, err := os.ReadFile(filepath.Join("..", "..", "cabac", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s is stale: run go run ./internal/gencabac", name)
		}
	}
}

func TestWriteRefusesAMissingDirectory(t *testing.T) {
	if err := write(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("writing into a directory that does not exist succeeded")
	}
	dir := t.TempDir()
	if err := write(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"decode_amd64.s", "decode_arm64.s"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Error(err)
		}
	}
}

func TestMainWritesIntoCabacAndExitsOnFailure(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "cabac"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	code := 0
	saved := osExit
	defer func() { osExit = saved }()
	osExit = func(c int) { code = c }

	main()
	if code != 0 {
		t.Fatalf("exit %d with the directory present", code)
	}
	if _, err := os.Stat(filepath.Join(root, "cabac", "decode_arm64.s")); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(root, "cabac")); err != nil {
		t.Fatal(err)
	}
	main()
	if code != 1 {
		t.Fatalf("exit %d, want 1 without the directory", code)
	}
}
