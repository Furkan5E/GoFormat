package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExitCodes(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.png")
	f, err := os.Create(good)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	f.Close()

	broken := filepath.Join(dir, "broken.png")
	os.WriteFile(broken, []byte("not a png"), 0o644)

	out := filepath.Join(dir, "out")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"success", []string{"-i", good, "-o", out, "-f", "png"}, exitOK},
		{"help", []string{"-h"}, exitOK},
		{"missing -i", []string{"-o", out}, exitUsage},
		{"unknown flag", []string{"-bogus"}, exitUsage},
		{"negative width", []string{"-i", good, "-o", out, "-width", "-5"}, exitUsage},
		{"quality too low", []string{"-i", good, "-o", out, "-q", "0"}, exitUsage},
		{"quality too high", []string{"-i", good, "-o", out, "-q", "101"}, exitUsage},
		{"no workers", []string{"-i", good, "-o", out, "-workers", "0"}, exitUsage},
		{"unknown format", []string{"-i", good, "-o", out, "-f", "xyz"}, exitUsage},
		{"missing input", []string{"-i", filepath.Join(dir, "nope.png"), "-o", out}, exitFailure},
		{"corrupt input", []string{"-i", broken, "-o", out}, exitFailure},
		{"batch with a failure", []string{"-i", dir, "-o", out, "-f", "png"}, exitFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.args); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

// errors used to go to stdout, where they mixed into piped output and were missed by 2> redirects
func TestRunWritesErrorsToStderr(t *testing.T) {
	dir := t.TempDir()
	stdout, err := os.Create(filepath.Join(dir, "stdout.txt"))
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.Create(filepath.Join(dir, "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	code := run([]string{"-i", filepath.Join(dir, "nope.png"), "-o", filepath.Join(dir, "out")})
	os.Stdout, os.Stderr = oldStdout, oldStderr
	stdout.Close()
	stderr.Close()

	if code != exitFailure {
		t.Fatalf("run = %d, want %d", code, exitFailure)
	}
	if got, _ := os.ReadFile(stderr.Name()); !strings.Contains(string(got), "Error accessing input path") {
		t.Errorf("stderr = %q, want the error message", got)
	}
	if got, _ := os.ReadFile(stdout.Name()); strings.Contains(string(got), "Error") {
		t.Errorf("stdout = %q, want no error message", got)
	}
}
