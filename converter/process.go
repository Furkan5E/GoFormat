package converter

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"

	"goformat/format"

	"golang.org/x/image/draw"
)

// Result describes a successful conversion
// warnings are returned rather than printed so batch mode can report them without breaking the progress bar
type Result struct {
	OutPath  string
	Warnings []string
}

func ProcessImage(ctx context.Context, inputPath string, outDir string, outFormat string, quality int, targetWidth int, targetHeight int, pixelart bool) (Result, error) {
	outPath := generateOutputPath(inputPath, outDir, strings.ToLower(outFormat))
	return ProcessImageTo(ctx, inputPath, outPath, outFormat, quality, targetWidth, targetHeight, pixelart)
}

// ProcessImageTo converts to an exact output path, for callers that choose the file name themselves
func ProcessImageTo(ctx context.Context, inputPath string, outPath string, outFormat string, quality int, targetWidth int, targetHeight int, pixelart bool) (Result, error) {
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}

	outFormat = strings.ToLower(outFormat)
	ext := strings.ToLower(filepath.Ext(inputPath))

	//extract historical metadata
	meta := extractMetadata(inputPath)

	if ext == ".gif" && outFormat == "gif" {
		//keep every frame instead of collapsing the animation to a single image
		if err := processGIFToGIF(inputPath, outPath, targetWidth, targetHeight, pixelart); err != nil {
			return Result{}, fmt.Errorf("failed to process %s: %w", inputPath, err)
		}
	} else {
		enc, err := format.GetEncoder(outFormat)
		if err != nil {
			return Result{}, err
		}

		img, err := loadImage(inputPath)
		if err != nil {
			return Result{}, fmt.Errorf("failed to load %s: %w", inputPath, err)
		}

		//the heic and avif decoders already turn the image themselves
		if ext != ".heic" && ext != ".heif" && ext != ".avif" {
			img = applyOrientation(img, meta.Orientation)
		}

		if targetWidth > 0 || targetHeight > 0 {
			img, err = resizeImage(img, targetWidth, targetHeight, pixelart)
			if err != nil {
				return Result{}, fmt.Errorf("failed to resize %s: %w", inputPath, err)
			}
		}

		if err := saveImage(img, outPath, enc, quality); err != nil {
			return Result{}, fmt.Errorf("failed to save %s: %w", outPath, err)
		}
	}

	res := Result{OutPath: outPath}

	//reinject metadata
	if meta.HasMetadata {
		if err := os.Chtimes(outPath, meta.Timestamp, meta.Timestamp); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("failed to preserve timestamp for %s: %v", outPath, err))
		}
	}

	return res, nil
}

func resizeImage(src image.Image, targetW, targetH int, pixelart bool) (image.Image, error) {
	bounds := src.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	//empty images are valid for some decoders (e.g. bmp) but cannot be scaled
	if origW == 0 || origH == 0 {
		return nil, fmt.Errorf("cannot resize empty %dx%d image", origW, origH)
	}

	//keep aspect ratio, rounding to nearest and never below 1px so thin images stay encodable
	if targetW == 0 {
		targetW = max(1, (origW*targetH+origH/2)/origH)
	}
	if targetH == 0 {
		targetH = max(1, (origH*targetW+origW/2)/origW)
	}

	//prevent massive allocations
	const maxDim = 16384
	if targetW > maxDim || targetH > maxDim {
		return nil, fmt.Errorf("target resolution %dx%d exceeds maximum limit of %d", targetW, targetH, maxDim)
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	if pixelart {
		draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	} else {
		draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	}

	return dst, nil
}

func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

func generateOutputPath(inputPath, outDir, targetFormat string) string {
	ext := filepath.Ext(inputPath)
	baseName := strings.TrimSuffix(filepath.Base(inputPath), ext)
	fileName := fmt.Sprintf("%s.%s", baseName, targetFormat)
	return filepath.Join(outDir, fileName)
}

func saveImage(img image.Image, path string, enc format.Encoder, quality int) error {
	return writeFile(path, func(w io.Writer) error {
		return enc.Encode(w, img, quality)
	})
}

// writeFile encodes into a temporary file next to outPath and renames it into place once it is complete
// a failed conversion then never leaves a partial file behind or damages a file that was already there
func writeFile(outPath string, encode func(w io.Writer) error) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(outPath), filepath.Base(outPath)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	if err = encode(tmp); err != nil {
		return err
	}
	//temporary files are created private to the user, unlike a normal output file
	if err = tmp.Chmod(0o644); err != nil {
		return err
	}
	//a close error means the data may not have reached the disk
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), outPath)
}
