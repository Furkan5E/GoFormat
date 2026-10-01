package format

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
)

func TestIsSupported(t *testing.T) {
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".tiff", ".tif", ".bmp", ".gif", ".ico", ".avif", ".heic", ".heif"} {
		if !IsSupported(ext) {
			t.Errorf("IsSupported(%q) = false, want true", ext)
		}
	}
	for _, ext := range []string{".txt", ".md", "", ".jxl"} {
		if IsSupported(ext) {
			t.Errorf("IsSupported(%q) = true, want false", ext)
		}
	}
}

func TestGetEncoder(t *testing.T) {
	for _, f := range []string{"jpeg", "jpg", "png", "webp", "tiff", "tif", "bmp", "gif", "ico", "avif"} {
		if _, err := GetEncoder(f); err != nil {
			t.Errorf("GetEncoder(%q) returned error: %v", f, err)
		}
	}
	//heic can be read but has no encoder
	for _, f := range []string{"xyz", "heic", "heif"} {
		if _, err := GetEncoder(f); err == nil {
			t.Errorf("GetEncoder(%q) returned no error", f)
		}
	}
}

func TestIcoEncodeRejectsLargeImages(t *testing.T) {
	var buf bytes.Buffer
	if err := (IcoEncoder{}).Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 257, 16)), 0); err == nil {
		t.Error("encoding a 257px wide ico returned no error")
	}
	if err := (IcoEncoder{}).Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 256, 256)), 0); err != nil {
		t.Errorf("encoding a 256x256 ico failed: %v", err)
	}
}

//every encoder should produce something that decodes back to the same size
func TestEncodersRoundTrip(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 12, 7))
	for i := range src.Pix {
		src.Pix[i] = uint8(i * 7)
	}

	for _, f := range []string{"jpeg", "png", "webp", "tiff", "bmp", "gif", "ico", "avif"} {
		t.Run(f, func(t *testing.T) {
			enc, _ := GetEncoder(f)
			var buf bytes.Buffer
			if err := enc.Encode(&buf, src, 85); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			img, name, err := image.Decode(&buf)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if img.Bounds() != src.Bounds() {
				t.Errorf("decoded %s bounds %v, want %v", name, img.Bounds(), src.Bounds())
			}
		})
	}
}

//jpeg and bmp cannot store transparency, which used to turn transparent areas black
func TestOpaqueFormatsUseWhiteBackground(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 16, 16))

	for _, f := range []string{"jpeg", "bmp"} {
		t.Run(f, func(t *testing.T) {
			enc, _ := GetEncoder(f)
			var buf bytes.Buffer
			if err := enc.Encode(&buf, src, 95); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			out, _, err := image.Decode(&buf)
			if err != nil {
				t.Fatalf("output does not decode: %v", err)
			}
			if r, g, b, _ := out.At(8, 8).RGBA(); r < 0xF000 || g < 0xF000 || b < 0xF000 {
				t.Errorf("transparent pixel became %v, want white", out.At(8, 8))
			}
		})
	}
}

func TestOnWhite(t *testing.T) {
	//opaque images are passed through untouched
	opaque := image.NewGray(image.Rect(0, 0, 4, 4))
	if got := onWhite(opaque); got != image.Image(opaque) {
		t.Error("opaque image was copied instead of returned as is")
	}

	//half transparent red over white is pink
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 128})
	got := color.NRGBAModel.Convert(onWhite(src).At(0, 0)).(color.NRGBA)
	if got.R != 255 || got.G < 120 || got.G > 135 || got.B != got.G || got.A != 255 {
		t.Errorf("half transparent red on white = %v, want about {255 127 127 255}", got)
	}
}

func TestPaletteBuilder(t *testing.T) {
	b := NewPaletteBuilder()
	red := color.NRGBA{255, 0, 0, 255}
	b.Add(red)
	b.Add(red)
	b.Add(color.NRGBA{0, 0, 255, 0}) //transparent, ignored

	pal, exact := b.Palette()
	if !exact {
		t.Fatal("exact = false for a two-colour palette")
	}
	if len(pal) != 2 {
		t.Fatalf("len(pal) = %d, want 2 (transparent + red)", len(pal))
	}
	if _, _, _, a := pal[TransparentIndex].RGBA(); a != 0 {
		t.Error("TransparentIndex entry is not transparent")
	}
	if pal[1] != red {
		t.Errorf("pal[1] = %v, want %v", pal[1], red)
	}
}

func TestPaletteBuilderFallback(t *testing.T) {
	b := NewPaletteBuilder()
	for i := 0; i < 300; i++ {
		b.Add(color.NRGBA{uint8(i), uint8(i >> 8), 0, 255})
	}
	if !b.Full() {
		t.Fatal("Full() = false after 300 colours")
	}

	pal, exact := b.Palette()
	if exact {
		t.Error("exact = true for the fallback palette")
	}
	if len(pal) != 256 {
		t.Errorf("len(pal) = %d, want 256", len(pal))
	}
	if _, _, _, a := pal[TransparentIndex].RGBA(); a != 0 {
		t.Error("fallback palette lost its transparent entry")
	}
}

func TestFlattenThreshold(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	src.SetNRGBA(0, 0, color.NRGBA{10, 20, 30, 127})
	src.SetNRGBA(1, 0, color.NRGBA{10, 20, 30, 128})
	src.SetNRGBA(2, 0, color.NRGBA{10, 20, 30, 255})

	flat := Flatten(src)
	if got := flat.NRGBAAt(0, 0); got != (color.NRGBA{}) {
		t.Errorf("alpha 127 became %v, want fully transparent", got)
	}
	for x := 1; x < 3; x++ {
		if got := flat.NRGBAAt(x, 0); got != (color.NRGBA{10, 20, 30, 255}) {
			t.Errorf("pixel %d became %v, want opaque {10 20 30 255}", x, got)
		}
	}
}

func TestQuantiseKeepsTransparency(t *testing.T) {
	odd := color.NRGBA{200, 100, 50, 255}
	flat := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			flat.SetNRGBA(x, y, odd)
		}
	}
	pal := color.Palette{color.NRGBA{}, odd}

	for _, dither := range []bool{false, true} {
		dst := Quantise(flat, pal, dither)
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				want := uint8(TransparentIndex)
				if x < 2 {
					want = 1
				}
				if got := dst.ColorIndexAt(x, y); got != want {
					t.Errorf("dither=%v: index at (%d,%d) = %d, want %d", dither, x, y, got, want)
				}
			}
		}
	}
}

//converting to gif used to go through Plan9, which has no transparent colour
func TestGifEncodeKeepsTransparencyAndColour(t *testing.T) {
	odd := color.NRGBA{200, 100, 50, 255}
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			src.SetNRGBA(x, y, odd)
		}
	}

	var buf bytes.Buffer
	if err := (GifEncoder{}).Encode(&buf, src, 0); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := gif.Decode(&buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Error("transparent corner came back opaque")
	}
	if got := color.NRGBAModel.Convert(img.At(3, 3)); got != odd {
		t.Errorf("centre colour = %v, want %v", got, odd)
	}
}
