package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
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
