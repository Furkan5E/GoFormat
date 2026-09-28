package batch

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//build a folder tree, writing a small png for each .png path and a text file for anything else
func makeTree(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, name := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(name, ".png") {
			err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 4, 4)))
		} else {
			_, err = f.WriteString("hello")
		}
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func inputPaths(jobs []Job) []string {
	var paths []string
	for _, j := range jobs {
		paths = append(paths, filepath.ToSlash(j.InputPath))
	}
	return paths
}

func TestCollectJobs(t *testing.T) {
	in := t.TempDir()
	out := filepath.Join(in, "output")
	makeTree(t, in, "a.png", "notes.txt", "sub/b.png", "sub/c.TIF", "output/old.png")

	jobs, skipped, err := collectJobs(context.Background(), in, out, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || skipped != 1 {
		t.Errorf("non-recursive: got jobs %v, skipped %d; want only a.png and 1 skipped", inputPaths(jobs), skipped)
	}

	jobs, skipped, err = collectJobs(context.Background(), in, out, true)
	if err != nil {
		t.Fatal(err)
	}
	//output/old.png must not be picked up, or -r keeps reconverting its own results
	if len(jobs) != 3 || skipped != 1 {
		t.Errorf("recursive: got jobs %v, skipped %d; want a.png, sub/b.png, sub/c.TIF and 1 skipped", inputPaths(jobs), skipped)
	}
	for _, j := range jobs {
		if strings.Contains(filepath.ToSlash(j.InputPath), "/output/") {
			t.Errorf("walked into the output folder: %s", j.InputPath)
		}
		if filepath.Base(j.InputPath) == "b.png" && j.OutputDir != filepath.Join(out, "sub") {
			t.Errorf("sub/b.png OutputDir = %q, want %q", j.OutputDir, filepath.Join(out, "sub"))
		}
	}
}

func TestProcessDirectory(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	makeTree(t, in, "a.png", "sub/b.png", "noimages/readme.txt")

	err := ProcessDirectory(context.Background(), in, out, Options{Format: "jpeg", Quality: 85, Recursive: true, Workers: 2})
	if err != nil {
		t.Fatalf("ProcessDirectory: %v", err)
	}

	for _, want := range []string{"a.jpeg", "sub/b.jpeg"} {
		if _, err := os.Stat(filepath.Join(out, want)); err != nil {
			t.Errorf("missing output %s", want)
		}
	}
	//folders without images should not get an empty copy in the output
	if _, err := os.Stat(filepath.Join(out, "noimages")); err == nil {
		t.Error("created an empty output folder for noimages")
	}
}

func TestProcessDirectoryReportsFailures(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	makeTree(t, in, "good.png")
	os.WriteFile(filepath.Join(in, "broken.png"), []byte("not a png"), 0o644)

	err := ProcessDirectory(context.Background(), in, out, Options{Format: "png", Quality: 85, Workers: 1})
	if err == nil {
		t.Fatal("expected an error when a file fails")
	}
	if _, statErr := os.Stat(filepath.Join(out, "good.png")); statErr != nil {
		t.Error("good file was not converted alongside the broken one")
	}
}

func TestProcessDirectoryCancelled(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	makeTree(t, in, "a.png")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ProcessDirectory(ctx, in, out, Options{Format: "png", Quality: 85, Workers: 1}); err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestProgressETA(t *testing.T) {
	p := newProgress(4)
	if got := p.eta(); got != "ETA --" {
		t.Errorf("eta before any job = %q, want \"ETA --\"", got)
	}
	p.done = 4
	if got := p.eta(); !strings.HasPrefix(got, "done in ") {
		t.Errorf("eta when finished = %q, want \"done in ...\"", got)
	}
}
