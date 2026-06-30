package buildrunner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeBuilderBin writes a tiny shell script that mimics `builder build <src>
// <out> <tag>`: it records its args and touches the out file.
func fakeBuilderBin(t *testing.T, argsFile string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "builder")
	script := "#!/bin/sh\n" +
		"echo \"$@\" > " + argsFile + "\n" +
		// args: build <src> <out> <tag> → $3 is out
		"echo rootfs > \"$3\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestBuildInvokesBuilderAndReturnsPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake not supported on windows")
	}
	argsFile := filepath.Join(t.TempDir(), "args")
	bin := fakeBuilderBin(t, argsFile)

	e := NewExec(bin, t.TempDir())
	out, err := e.Build(context.Background(), "/src/web", "antariksh/web:latest")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected rootfs at %s: %v", out, err)
	}
	if !strings.HasSuffix(out, "antariksh-web-latest.ext4") {
		t.Errorf("out path = %s, want suffix antariksh-web-latest.ext4", out)
	}

	got, _ := os.ReadFile(argsFile)
	args := strings.Fields(string(got))
	if len(args) < 4 || args[0] != "build" || args[1] != "/src/web" || args[3] != "antariksh/web:latest" {
		t.Errorf("builder args = %v", args)
	}
}

func TestBuildPropagatesFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake not supported on windows")
	}
	bin := filepath.Join(t.TempDir(), "builder")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho boom >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := NewExec(bin, t.TempDir())
	if _, err := e.Build(context.Background(), "/src/web", "t"); err == nil {
		t.Fatal("expected build error")
	}
}

func TestSanitizeTag(t *testing.T) {
	if got := sanitizeTag("antariksh/web:latest"); got != "antariksh-web-latest" {
		t.Errorf("sanitizeTag = %s", got)
	}
}
