package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadSecretFile(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"abc", "abc"},
		{"abc\n", "abc"},
		{"abc\r\n", "abc"},
		{"  abc \n\n", "abc"},
		{"a b\nc\n", "a b\nc"},
		{"\n", ""},
		{"", ""},
	} {
		got, err := readSecretFile(writeTemp(t, tc.in))
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestReadSecretFileMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope")
	_, err := readSecretFile(p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("error should name path: %v", err)
	}
}

func TestResolveSecret(t *testing.T) {
	file := writeTemp(t, "from-file\n")
	envFile := writeTemp(t, "from-env\n")
	empty := writeTemp(t, "\n")
	missing := filepath.Join(t.TempDir(), "nope")

	tests := []struct {
		name       string
		value      string
		valueSet   bool
		fileFlag   string
		envFile    string
		allowEmpty bool
		want       string
		wantErr    bool
	}{
		{name: "nothing set", want: ""},
		{name: "value only", value: "v", valueSet: true, want: "v"},
		{name: "file only", fileFlag: file, want: "from-file"},
		{name: "env file only", envFile: envFile, want: "from-env"},
		{name: "file beats env", fileFlag: file, envFile: envFile, want: "from-file"},
		{name: "value beats env file", value: "v", valueSet: true, envFile: envFile, want: "v"},
		{name: "value and file conflict", value: "v", valueSet: true, fileFlag: file, wantErr: true},
		{name: "unreadable file", fileFlag: missing, wantErr: true},
		{name: "unreadable env file", envFile: missing, wantErr: true},
		{name: "empty file rejected", fileFlag: empty, wantErr: true},
		{name: "empty file allowed", fileFlag: empty, allowEmpty: true, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSecret("x", tc.value, tc.valueSet, tc.fileFlag, tc.envFile, tc.allowEmpty)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestResolveSecretErrorHasNoContent(t *testing.T) {
	_, err := resolveSecret("x", "supersecret", true, writeTemp(t, "other-secret"), "", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "supersecret") || strings.Contains(err.Error(), "other-secret") {
		t.Errorf("error leaks content: %v", err)
	}
}
