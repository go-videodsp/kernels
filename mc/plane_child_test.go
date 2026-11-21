package mc

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"strings"
	"time"
)

// A refusal that is missing hands the assembly a block it was never written
// for, and the assembly does not fail politely: it writes outside its buffers,
// the process dies of a signal (and a Go process that dies inside a signal
// handler can hang), or it loops. A test that sends refused blocks therefore
// runs in a child process that the parent judges: exit status, output and a
// deadline, all asserted. The child is this very test binary, selected by
// -test.run and marked by an environment variable.

const planeChildEnv = "MC_PLANE_CHILD"

type childTB interface {
	Helper()
	Name() string
	Fatalf(format string, args ...any)
}

// inPlaneChild reports whether this process is the child that must run the
// body. In the parent it runs the child, asserts its verdict, and reports false.
// While fuzzing it reports true: a crash is then the finding itself.
func inPlaneChild(t childTB) bool {
	t.Helper()
	if os.Getenv(planeChildEnv) == "1" {
		return true
	}
	if f := flag.Lookup("test.fuzz"); f != nil && f.Value.String() != "" {
		return true
	}
	name := t.Name()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.v", "-test.timeout=80s", "-test.run=^"+name+"$")
	cmd.Env = append(os.Environ(), planeChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the child process of %s failed (%v); a refused block reached the kernels?\n%s", name, err, tail(out))
	}
	if !strings.Contains(string(out), "--- PASS: "+name+" ") && !strings.Contains(string(out), "--- PASS: "+name+"\n") {
		t.Fatalf("the child process of %s did not report the test as passed (selected nothing?)\n%s", name, tail(out))
	}
	return false
}

func tail(b []byte) string {
	if len(b) > 4000 {
		b = b[len(b)-4000:]
	}
	return string(b)
}
