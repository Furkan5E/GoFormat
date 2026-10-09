package converter

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
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
				got, err := resizeImage(src, Options{Width: tt.w, Height: tt.h, Pixelart: pixelart})
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

// an empty bmp used to panic with an integer divide by zero
func TestResizeImageEmpty(t *testing.T) {
	if _, err := resizeImage(image.NewRGBA(image.Rect(0, 0, 0, 0)), Options{Width: 100}); err == nil {
		t.Error("resizing a 0x0 image returned no error")
	}
}

func TestResizeImageTooLarge(t *testing.T) {
	if _, err := resizeImage(image.NewRGBA(image.Rect(0, 0, 10, 10)), Options{Width: 20000}); err == nil {
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

	res, err := ProcessImage(context.Background(), in, dir, Options{Format: "jpeg", Quality: 85, Width: 10})
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

// a failed encode used to leave a 0-byte file, or truncate one that was already there
func TestProcessImageFailureLeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 600, 600)))
	out := filepath.Join(dir, "a.ico")

	//ico cannot hold a 600x600 image, so the encoder fails after the output file is opened
	if _, err := ProcessImage(context.Background(), in, dir, Options{Format: "ico", Quality: 85}); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("failed conversion left an output file behind")
	}

	os.WriteFile(out, []byte("existing"), 0o644)
	if _, err := ProcessImage(context.Background(), in, dir, Options{Format: "ico", Quality: 85}); err == nil {
		t.Fatal("expected an error")
	}
	if data, _ := os.ReadFile(out); string(data) != "existing" {
		t.Errorf("failed conversion changed the existing output to %q", data)
	}

	//a successful conversion replaces the existing file
	if _, err := ProcessImage(context.Background(), in, dir, Options{Format: "ico", Quality: 85, Width: 64}); err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	if data, _ := os.ReadFile(out); string(data) == "existing" {
		t.Error("successful conversion did not replace the existing output")
	}

	if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(left) > 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

func TestApplyOrientation(t *testing.T) {
	//a 3x2 image with a different value in every pixel:
	//  1 2 3
	//  4 5 6
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for i := 0; i < 6; i++ {
		src.SetRGBA(i%3, i/3, color.RGBA{uint8(i + 1), 0, 0, 255})
	}

	tests := []struct {
		orientation int
		want        [][]uint8
	}{
		{0, [][]uint8{{1, 2, 3}, {4, 5, 6}}},
		{1, [][]uint8{{1, 2, 3}, {4, 5, 6}}},
		{2, [][]uint8{{3, 2, 1}, {6, 5, 4}}},
		{3, [][]uint8{{6, 5, 4}, {3, 2, 1}}},
		{4, [][]uint8{{4, 5, 6}, {1, 2, 3}}},
		{5, [][]uint8{{1, 4}, {2, 5}, {3, 6}}},
		{6, [][]uint8{{4, 1}, {5, 2}, {6, 3}}},
		{7, [][]uint8{{6, 3}, {5, 2}, {4, 1}}},
		{8, [][]uint8{{3, 6}, {2, 5}, {1, 4}}},
		{9, [][]uint8{{1, 2, 3}, {4, 5, 6}}},
	}
	for _, tt := range tests {
		got := applyOrientation(src, tt.orientation)
		if b := got.Bounds(); b.Dx() != len(tt.want[0]) || b.Dy() != len(tt.want) {
			t.Errorf("orientation %d: got %dx%d, want %dx%d", tt.orientation, b.Dx(), b.Dy(), len(tt.want[0]), len(tt.want))
			continue
		}
		for y, row := range tt.want {
			for x, want := range row {
				if r, _, _, _ := got.At(x, y).RGBA(); uint8(r>>8) != want {
					t.Errorf("orientation %d: pixel (%d,%d) = %d, want %d", tt.orientation, x, y, r>>8, want)
				}
			}
		}
	}
}

// write a jpeg carrying only an EXIF orientation tag, the way a phone stores a sideways photo
func writeOrientedJPEG(t *testing.T, path string, img image.Image, orientation byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	exifSegment := []byte{
		0xFF, 0xE1, 0x00, 0x22, //APP1 marker and length
		'E', 'x', 'i', 'f', 0, 0,
		'M', 'M', 0, 42, 0, 0, 0, 8, //big-endian TIFF header
		0, 1, //one tag
		0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, orientation, 0, 0, //orientation, one short
		0, 0, 0, 0, //no further tags
	}
	//the segment goes straight after the two-byte start of image marker
	data := append([]byte{}, buf.Bytes()[:2]...)
	data = append(data, exifSegment...)
	data = append(data, buf.Bytes()[2:]...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// a photo stored on its side used to stay on its side once the tag was dropped
func TestProcessImageAppliesOrientation(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "photo.jpg")

	//stored 32x16 with the left half red, to be shown turned a quarter clockwise: 16x32 with the top half red
	src := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			if x < 16 {
				src.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
			} else {
				src.SetRGBA(x, y, color.RGBA{0, 0, 255, 255})
			}
		}
	}
	writeOrientedJPEG(t, in, src, 6)

	res, err := ProcessImage(context.Background(), in, dir, Options{Format: "png", Quality: 85})
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	out, err := loadImage(res.OutPath)
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 16 || b.Dy() != 32 {
		t.Fatalf("output is %dx%d, want 16x32", b.Dx(), b.Dy())
	}
	if r, _, b, _ := out.At(8, 4).RGBA(); r < 0xC000 || b > 0x4000 {
		t.Errorf("top of the output is not red: %v", out.At(8, 4))
	}
	if r, _, b, _ := out.At(8, 28).RGBA(); b < 0xC000 || r > 0x4000 {
		t.Errorf("bottom of the output is not blue: %v", out.At(8, 28))
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
			if _, err := ProcessImage(tt.ctx, tt.input, dir, Options{Format: tt.format, Quality: 85}); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// errors keep their cause so callers can tell a missing file from a corrupt one
func TestProcessImageErrorsKeepCause(t *testing.T) {
	dir := t.TempDir()
	_, err := ProcessImage(context.Background(), filepath.Join(dir, "nope.png"), dir, Options{Format: "png", Quality: 85})
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want it to wrap os.ErrNotExist", err)
	}
}

// two frames of an 8x8 sprite: a square in an off-Plan9 colour that moves diagonally
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

	if _, _, err := processGIFToGIF(in, out, Options{}); err != nil {
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

		if _, _, err := processGIFToGIF(in, out, Options{Width: 16, Pixelart: pixelart}); err != nil {
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

// converting a file into its own folder and format used to replace the original with the converted copy
func TestProcessImageRefusesToOverwriteInput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 20, 10)))
	before, _ := os.ReadFile(in)

	_, err := ProcessImage(context.Background(), in, dir, Options{Format: "png", Quality: 85, Width: 10})
	if !errors.Is(err, ErrOverwritesInput) {
		t.Fatalf("err = %v, want ErrOverwritesInput", err)
	}
	if after, _ := os.ReadFile(in); !bytes.Equal(before, after) {
		t.Error("the input file was changed")
	}
}

func TestProcessImageReportsReplacedFiles(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 4, 4)))

	res, err := ProcessImage(context.Background(), in, dir, Options{Format: "jpeg", Quality: 85})
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	if res.Replaced {
		t.Error("Replaced = true for a new file")
	}

	res, err = ProcessImage(context.Background(), in, dir, Options{Format: "jpeg", Quality: 85})
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	if !res.Replaced {
		t.Error("Replaced = false when the output already existed")
	}
}

func TestProcessImageSkipExisting(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	out := filepath.Join(dir, "a.jpeg")
	opts := Options{Format: "jpeg", Quality: 85, SkipExisting: true}

	//nothing to skip yet, so the file is converted as usual
	res, err := ProcessImage(context.Background(), in, dir, opts)
	if err != nil || res.Skipped {
		t.Fatalf("first run: Skipped = %v, err = %v", res.Skipped, err)
	}

	os.WriteFile(out, []byte("existing"), 0o644)
	res, err = ProcessImage(context.Background(), in, dir, opts)
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	if !res.Skipped || res.Replaced {
		t.Errorf("Skipped = %v, Replaced = %v, want true and false", res.Skipped, res.Replaced)
	}
	if data, _ := os.ReadFile(out); string(data) != "existing" {
		t.Errorf("existing output was changed to %q", data)
	}

	//converting onto the input is an error without the flag, with it the file is just skipped
	opts.Format = "png"
	if res, err := ProcessImage(context.Background(), in, dir, opts); err != nil || !res.Skipped {
		t.Errorf("same file: Skipped = %v, err = %v, want it skipped", res.Skipped, err)
	}
}

func TestProcessImageDryRun(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	out := filepath.Join(dir, "a.jpeg")
	opts := Options{Format: "jpeg", Quality: 85, DryRun: true}

	res, err := ProcessImage(context.Background(), in, dir, opts)
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	if !res.DryRun || res.Replaced || res.OutPath != out {
		t.Errorf("got %+v, want a dry run result for %s", res, out)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("dry run wrote the output file")
	}

	//an existing output is reported as one that would be replaced, and is left as it is
	os.WriteFile(out, []byte("existing"), 0o644)
	res, err = ProcessImage(context.Background(), in, dir, opts)
	if err != nil || !res.Replaced {
		t.Errorf("Replaced = %v, err = %v, want true and nil", res.Replaced, err)
	}
	if data, _ := os.ReadFile(out); string(data) != "existing" {
		t.Errorf("dry run changed the existing output to %q", data)
	}

	//problems a real run would hit are still reported
	opts.Format = "png"
	if _, err := ProcessImage(context.Background(), in, dir, opts); !errors.Is(err, ErrOverwritesInput) {
		t.Errorf("err = %v, want ErrOverwritesInput", err)
	}
}

// huge inputs used to be decoded whatever their size, on every worker at once
func TestProcessImageMaxPixels(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 200, 100)))

	_, err := ProcessImage(context.Background(), in, dir, Options{Format: "jpeg", Quality: 85, MaxPixels: 19_999})
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.jpeg")); err == nil {
		t.Error("an output was written for a rejected image")
	}

	for _, limit := range []int64{20_000, 0} {
		if _, err := ProcessImage(context.Background(), in, dir, Options{Format: "jpeg", Quality: 85, MaxPixels: limit}); err != nil {
			t.Errorf("MaxPixels %d: %v", limit, err)
		}
	}
}

// the size check reads only the header, which every readable format has to support
func TestProcessImageReadsEveryFormat(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	writePNG(t, in, image.NewRGBA(image.Rect(0, 0, 20, 10)))

	for _, f := range []string{"jpeg", "webp", "tiff", "bmp", "gif", "ico", "avif"} {
		first, err := ProcessImage(context.Background(), in, dir, Options{Format: f, Quality: 85})
		if err != nil {
			t.Fatalf("png to %s: %v", f, err)
		}
		back, err := ProcessImageTo(context.Background(), first.OutPath, filepath.Join(dir, f+".png"), Options{Format: "png", MaxPixels: 200})
		if err != nil {
			t.Fatalf("%s to png: %v", f, err)
		}
		if back.SrcSize != image.Pt(20, 10) {
			t.Errorf("%s read back as %v, want 20x10", f, back.SrcSize)
		}
	}
}

func TestBudget(t *testing.T) {
	b := NewBudget(100)
	ctx := context.Background()

	first, err := b.Acquire(ctx, 60)
	if err != nil || first != 60 {
		t.Fatalf("Acquire(60) = %d, %v", first, err)
	}

	//60 more do not fit until the first 60 are released
	got := make(chan int64)
	go func() {
		n, _ := b.Acquire(ctx, 60)
		got <- n
	}()
	select {
	case <-got:
		t.Fatal("second Acquire did not wait for room")
	case <-time.After(50 * time.Millisecond):
	}
	b.Release(first)
	if n := <-got; n != 60 {
		t.Fatalf("second Acquire = %d, want 60", n)
	}
	b.Release(60)

	//an image larger than the whole budget takes all of it instead of waiting forever
	if n, err := b.Acquire(ctx, 500); err != nil || n != 100 {
		t.Fatalf("Acquire(500) = %d, %v, want 100", n, err)
	}

	//a cancelled run stops waiting
	cancelled, cancel := context.WithCancel(ctx)
	done := make(chan error)
	go func() {
		_, err := b.Acquire(cancelled, 10)
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire kept waiting after the context was cancelled")
	}
}

// draw an animation the way a viewer does, returning the finished picture of every frame
func renderGIF(g *gif.GIF) []*image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	var shown []*image.RGBA
	for i, frame := range g.Image {
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		snapshot := image.NewRGBA(canvas.Bounds())
		draw.Draw(snapshot, snapshot.Bounds(), canvas, image.Point{}, draw.Src)
		shown = append(shown, snapshot)
		if g.Disposal[i] == gif.DisposalBackground {
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		}
	}
	return shown
}

// three frames with a palette of their own each, 600 colours in total, the later two covering only part of the canvas
func writePatchworkGIF(t *testing.T, path string) {
	t.Helper()
	g := &gif.GIF{Config: image.Config{Width: 20, Height: 10}}
	rects := []image.Rectangle{image.Rect(0, 0, 20, 10), image.Rect(3, 2, 13, 9), image.Rect(10, 0, 20, 10)}
	for i, rect := range rects {
		pal := color.Palette{color.NRGBA{}}
		for c := 0; c < 200; c++ {
			pal = append(pal, color.NRGBA{uint8(c), uint8(i * 100), uint8(255 - c), 255})
		}
		frame := image.NewPaletted(rect, pal)
		for n := range frame.Pix {
			//every seventh pixel is left transparent so the frame underneath shows through
			if n%7 != 0 {
				frame.Pix[n] = uint8(1 + (n*13)%200)
			}
		}
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 5)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	g.Disposal[1] = gif.DisposalBackground

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatal(err)
	}
}

// resizing with -pixel used to redraw every frame and match it to a new palette, losing colours when there were over 255
func TestProcessGIFToGIFPixelResizeKeepsEveryColour(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.gif"), filepath.Join(dir, "out.gif")
	writePatchworkGIF(t, in)

	if _, _, err := processGIFToGIF(in, out, Options{Width: 60, Pixelart: true}); err != nil {
		t.Fatal(err)
	}

	src, got := readGIF(t, in), readGIF(t, out)
	if got.Config.Width != 60 || got.Config.Height != 30 {
		t.Fatalf("output is %dx%d, want 60x30", got.Config.Width, got.Config.Height)
	}
	if len(got.Image) != len(src.Image) {
		t.Fatalf("got %d frames, want %d", len(got.Image), len(src.Image))
	}

	//every frame must look exactly like the original scaled up three times
	want, shown := renderGIF(src), renderGIF(got)
	for i := range want {
		for y := 0; y < 30; y++ {
			for x := 0; x < 60; x++ {
				if shown[i].RGBAAt(x, y) != want[i].RGBAAt(x/3, y/3) {
					t.Fatalf("frame %d pixel (%d,%d) = %v, want %v", i, x, y, shown[i].RGBAAt(x, y), want[i].RGBAAt(x/3, y/3))
				}
			}
		}
	}
}

// shrinking can leave a small frame with nothing in it, which falls back to redrawing the frames
func TestProcessGIFToGIFPixelShrink(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.gif"), filepath.Join(dir, "out.gif")

	pal := color.Palette{color.NRGBA{}, spriteColour}
	g := &gif.GIF{Config: image.Config{Width: 16, Height: 16}}
	for _, rect := range []image.Rectangle{image.Rect(0, 0, 16, 16), image.Rect(0, 0, 1, 1)} {
		frame := image.NewPaletted(rect, pal)
		for n := range frame.Pix {
			frame.Pix[n] = 1
		}
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 5)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	f, err := os.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if _, _, err := processGIFToGIF(in, out, Options{Width: 4, Pixelart: true}); err != nil {
		t.Fatal(err)
	}
	got := readGIF(t, out)
	if len(got.Image) != 2 {
		t.Fatalf("got %d frames, want 2", len(got.Image))
	}
	for i, shown := range renderGIF(&gif.GIF{Image: got.Image, Disposal: got.Disposal, Config: image.Config{Width: 4, Height: 4}}) {
		if got := color.NRGBAModel.Convert(shown.At(2, 2)); got != spriteColour {
			t.Errorf("frame %d centre = %v, want %v", i, got, spriteColour)
		}
	}
}

// smooth scaling needs a new palette; with too many colours for one it used to fall back to a fixed, poorly matched one
func TestProcessGIFToGIFSmoothResizeManyColours(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.gif"), filepath.Join(dir, "out.gif")
	writePatchworkGIF(t, in)

	if _, _, err := processGIFToGIF(in, out, Options{Width: 10}); err != nil {
		t.Fatal(err)
	}
	got := readGIF(t, out)
	if len(got.Image) != 3 {
		t.Fatalf("got %d frames, want 3", len(got.Image))
	}
	for i, frame := range got.Image {
		if b := frame.Bounds(); b.Dx() != 10 || b.Dy() != 5 {
			t.Errorf("frame %d is %dx%d, want 10x5", i, b.Dx(), b.Dy())
		}
	}
}
