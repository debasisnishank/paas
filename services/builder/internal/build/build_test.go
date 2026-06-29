package build

import (
	"strings"
	"testing"
)

func TestBuildInitScript(t *testing.T) {
	got := buildInitScript(ImageConfig{
		Entrypoint: nil,
		Cmd:        []string{"/server"},
		Env:        []string{"FOO=bar", "PORT=80"},
		WorkingDir: "/app",
	})
	for _, want := range []string{
		"#!/bin/sh\n",
		"cd '/app'\n",
		"export 'FOO=bar'\n",
		"export 'PORT=80'\n",
		"exec '/server'\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("init script missing %q\n---\n%s", want, got)
		}
	}
}

func TestBuildInitScriptEntrypointPlusCmd(t *testing.T) {
	got := buildInitScript(ImageConfig{
		Entrypoint: []string{"/bin/app", "--serve"},
		Cmd:        []string{"--port", "80"},
	})
	if !strings.Contains(got, "exec '/bin/app' '--serve' '--port' '80'\n") {
		t.Errorf("unexpected exec line:\n%s", got)
	}
}

func TestBuildInitScriptDefaultsToShell(t *testing.T) {
	got := buildInitScript(ImageConfig{})
	if !strings.Contains(got, "exec '/bin/sh'\n") {
		t.Errorf("expected /bin/sh fallback:\n%s", got)
	}
}

func TestShQuoteEscapesQuotes(t *testing.T) {
	if got := shQuote("a'b"); got != `'a'\''b'` {
		t.Errorf("shQuote = %s", got)
	}
}

func TestParseLeadingInt(t *testing.T) {
	n, err := parseLeadingInt("123\t/some/dir\n")
	if err != nil || n != 123 {
		t.Fatalf("parseLeadingInt = %d, %v", n, err)
	}
	if _, err := parseLeadingInt("/no/digits"); err == nil {
		t.Error("expected error for no leading int")
	}
}
