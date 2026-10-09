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
		{"version", []string{"-version"}, exitOK},
		{"missing -i", []string{"-o", out}, exitUsage},
		{"unknown flag", []string{"-bogus"}, exitUsage},
		{"negative width", []string{"-i", good, "-o", out, "-width", "-5"}, exitUsage},
		{"quality too low", []string{"-i", good, "-o", out, "-q", "0"}, exitUsage},
		{"quality too high", []string{"-i", good, "-o", out, "-q", "101"}, exitUsage},
		{"no workers", []string{"-i", good, "-o", out, "-workers", "0"}, exitUsage},
		{"unknown format", []string{"-i", good, "-o", out, "-f", "xyz"}, exitUsage},
		{"missing input", []string{"-i", filepath.Join(dir, "nope.png"), "-o", out}, exitFailure},
		{"corrupt input", []string{"-i", broken, "-o", out}, exitFailure},
		{"dry run", []string{"-i", good, "-o", filepath.Join(dir, "dry"), "-f", "png", "-dry-run"}, exitOK},
		{"verbose and quiet together", []string{"-i", good, "-o", out, "-v", "-quiet"}, exitUsage},
		{"lossless jpeg", []string{"-i", good, "-o", out, "-f", "jpeg", "-lossless"}, exitUsage},
		{"lossless webp", []string{"-i", good, "-o", out, "-f", "webp", "-lossless"}, exitOK},
		{"negative megapixels", []string{"-i", good, "-o", out, "-max-megapixels", "-1"}, exitUsage},
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

// -quiet leaves stdout empty, -v describes each file
func TestRunVerbosity(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	os.Mkdir(in, 0o755)
	f, err := os.Create(filepath.Join(in, "a.png"))
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 8, 4)))
	f.Close()

	capture := func(args ...string) string {
		t.Helper()
		stdout, err := os.Create(filepath.Join(dir, "stdout.txt"))
		if err != nil {
			t.Fatal(err)
		}
		old := os.Stdout
		os.Stdout = stdout
		code := run(args)
		os.Stdout = old
		stdout.Close()
		if code != exitOK {
			t.Fatalf("run(%v) = %d, want %d", args, code, exitOK)
		}
		got, _ := os.ReadFile(stdout.Name())
		return string(got)
	}

	for _, input := range []string{in, filepath.Join(in, "a.png")} {
		if got := capture("-i", input, "-o", filepath.Join(dir, "out"), "-f", "png", "-width", "4", "-quiet"); got != "" {
			t.Errorf("-quiet printed %q, want nothing", got)
		}
		if got := capture("-i", input, "-o", filepath.Join(dir, "out"), "-f", "png", "-width", "4", "-v"); !strings.Contains(got, "8x4 -> 4x2") {
			t.Errorf("-v printed %q, want the sizes before and after", got)
		}
	}
}
