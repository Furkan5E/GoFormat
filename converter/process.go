package converter

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"context"

	"goformat/format"

	"golang.org/x/image/draw"
)

//Result describes a successful conversion
//warnings are returned rather than printed so batch mode can report them without breaking the progress bar
type Result struct {
	OutPath  string
	Warnings []string
}

func ProcessImage(ctx context.Context, inputPath string, outDir string, outFormat string, quality int, targetWidth int, targetHeight int, pixelart bool) (Result, error) {
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	
	outFormat = strings.ToLower(outFormat)
	ext := strings.ToLower(filepath.Ext(inputPath))

	//extract historical metadata
	meta := extractMetadata(inputPath)
	outPath := generateOutputPath(inputPath, outDir, outFormat)

	if ext == ".gif" && outFormat == "gif" {
		//keep every frame instead of collapsing the animation to a single image
		if err := processGIFToGIF(inputPath, outPath, targetWidth, targetHeight, pixelart); err != nil {
			return Result{}, fmt.Errorf("failed to process %s: %v", inputPath, err)
		}
	} else {
		enc, err := format.GetEncoder(outFormat)
		if err != nil {
			return Result{}, err
		}

		img, err := loadImage(inputPath)
		if err != nil {
			return Result{}, fmt.Errorf("failed to load %s: %v", inputPath, err)
		}

		if targetWidth > 0 || targetHeight > 0 {
			img, err = resizeImage(img, targetWidth, targetHeight, pixelart)
			if err != nil {
				return Result{}, fmt.Errorf("failed to resize %s: %v", inputPath, err)
			}
		}

		if err := saveImage(img, outPath, enc, quality); err != nil {
			return Result{}, fmt.Errorf("failed to save %s: %v", outPath, err)
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
	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return enc.Encode(outFile, img, quality)
}