package converter

import (
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestResizeImage(t *testing.T) {
	tests := []struct {
		name         string
		srcW, srcH   int
		w, h         int
		wantW, wantH int
	}{
		{"width only keeps aspect", 220, 220, 150, 0, 150, 150},
		{"height only keeps aspect", 400, 200, 0, 100, 200, 100},
		{"both given", 100, 100, 30, 70, 30, 70},
		{"rounds to nearest", 10, 3, 5, 0, 5, 2},
		{"thin image never rounds to 0", 1000, 1, 10, 0, 10, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, tt.srcW, tt.srcH))
			for _, pixelart := range []bool{false, true} {
				got, err := resizeImage(src, tt.w, tt.h, pixelart)
				if err != nil {
					t.Fatalf("resizeImage: %v", err)
				}
				if b := got.Bounds(); b.Dx() != tt.wantW || b.Dy() != tt.wantH {
					t.Errorf("pixelart=%v: got %dx%d, want %dx%d", pixelart, b.Dx(), b.Dy(), tt.wantW, tt.wantH)
				}
			}
		})
	}
}

//an empty bmp used to panic with an integer divide by zero
func TestResizeImageEmpty(t *testing.T) {
	if _, err := resizeImage(image.NewRGBA(image.Rect(0, 0, 0, 0)), 100, 0, false); err == nil {
		t.Error("resizing a 0x0 image returned no error")
	}
}

func TestResizeImageTooLarge(t *testing.T) {
	if _, err := resizeImage(image.NewRGBA(image.Rect(0, 0, 10, 10)), 20000, 0, false); err == nil {
		t.Error("resizing beyond the maximum dimension returned no error")
	}
}

func TestGenerateOutputPath(t *testing.T) {
	got := generateOutputPath(filepath.Join("in", "photo.final.PNG"), "out", "jpeg")
	if want := filepath.Join("out", "photo.final.jpeg"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProcessImage(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 20, 10)))

	res, err := ProcessImage(context.Background(), in, dir, "jpeg", 85, 10, 0, false)
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	if want := filepath.Join(dir, "a.jpeg"); res.OutPath != want {
		t.Errorf("OutPath = %q, want %q", res.OutPath, want)
	}

	f, err := os.Open(res.OutPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
	if cfg.Width != 10 || cfg.Height != 5 {
		t.Errorf("output is %dx%d, want 10x5", cfg.Width, cfg.Height)
	}
}

func TestProcessImageErrors(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 4, 4)))

	broken := filepath.Join(dir, "broken.png")
	os.WriteFile(broken, []byte("not a png"), 0o644)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name   string
		ctx    context.Context
		input  string
		format string
	}{
		{"unknown format", context.Background(), in, "xyz"},
		{"corrupt input", context.Background(), broken, "png"},
		{"missing input", context.Background(), filepath.Join(dir, "nope.png"), "png"},
		{"cancelled", cancelled, in, "png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ProcessImage(tt.ctx, tt.input, dir, tt.format, 85, 0, 0, false); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

//two frames of an 8x8 sprite: a square in an off-Plan9 colour that moves diagonally
var spriteColour = color.NRGBA{200, 100, 50, 255}

func writeMovingSprite(t *testing.T, path string) {
	t.Helper()
	pal := color.Palette{color.NRGBA{}, spriteColour}
	g := &gif.GIF{LoopCount: 0}
	for i := 0; i < 2; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				frame.SetColorIndex(x+i*4, y+i*4, 1)
			}
		}
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 10+i)
		g.Disposal = append(g.Disposal, gif.DisposalBackground)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatal(err)
	}
}

func readGIF(t *testing.T, path string) *gif.GIF {
	t.Helper()
	g, err := loadGIF(path)
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
	return g
}

func TestProcessGIFToGIFKeepsFrames(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.gif"), filepath.Join(dir, "out.gif")
	writeMovingSprite(t, in)

	if err := processGIFToGIF(in, out, 0, 0, false); err != nil {
		t.Fatal(err)
	}

	g := readGIF(t, out)
	if len(g.Image) != 2 {
		t.Fatalf("got %d frames, want 2", len(g.Image))
	}
	if g.Delay[0] != 10 || g.Delay[1] != 11 {
		t.Errorf("delays = %v, want [10 11]", g.Delay)
	}
	if g.LoopCount != 0 {
		t.Errorf("LoopCount = %d, want 0", g.LoopCount)
	}
}

func TestProcessGIFToGIFResize(t *testing.T) {
	for _, pixelart := range []bool{true, false} {
		dir := t.TempDir()
		in, out := filepath.Join(dir, "in.gif"), filepath.Join(dir, "out.gif")
		writeMovingSprite(t, in)

		if err := processGIFToGIF(in, out, 16, 0, pixelart); err != nil {
			t.Fatal(err)
		}

		g := readGIF(t, out)
		for i, frame := range g.Image {
			if b := frame.Bounds(); b.Dx() != 16 || b.Dy() != 16 {
				t.Fatalf("pixelart=%v frame %d is %dx%d, want 16x16", pixelart, i, b.Dx(), b.Dy())
			}
			if g.Disposal[i] != gif.DisposalBackground {
				t.Errorf("pixelart=%v frame %d disposal = %d, want background", pixelart, i, g.Disposal[i])
			}

			//each frame holds only its own square: 8x8 at 2x scale, no leftover from the previous frame
			opaque := 0
			for _, idx := range frame.Pix {
				if _, _, _, a := frame.Palette[idx].RGBA(); a != 0 {
					opaque++
					if got := color.NRGBAModel.Convert(frame.Palette[idx]); got != spriteColour {
						t.Fatalf("pixelart=%v frame %d has colour %v, want %v", pixelart, i, got, spriteColour)
					}
				}
			}
			if opaque != 64 {
				t.Errorf("pixelart=%v frame %d has %d opaque pixels, want 64", pixelart, i, opaque)
			}
		}
	}
}
