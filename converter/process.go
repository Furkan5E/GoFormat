package converter

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"goformat/format"

	"golang.org/x/image/draw"
)

func ProcessImage(inputPath string, outDir string, outFormat string, quality int, targetWidth int, targetHeight int) error {
	outFormat = strings.ToLower(outFormat)
	enc, err := format.GetEncoder(outFormat)
	if err != nil {
		return err
	}

	//extract historical metadata
	meta := extractMetadata(inputPath)

	img, err := loadImage(inputPath)
	if err != nil {
		return fmt.Errorf("failed to load %s: %v", inputPath, err)
	}

	if targetWidth > 0 || targetHeight > 0 {
		img = resizeImage(img, targetWidth, targetHeight)
	}

	outPath := generateOutputPath(inputPath, outDir, outFormat)

	err = saveImage(img, outPath, enc, quality)
	if err != nil {
		return fmt.Errorf("failed to save %s: %v", outPath, err)
	}

	//reinject metadata
	if meta.HasMetadata {
		err = os.Chtimes(outPath, meta.Timestamp, meta.Timestamp)
		if err != nil {
			fmt.Printf("Warning: Failed to preserve timestamp for %s\n", outPath)
		}
	}

	fmt.Printf("Saved converted file as: %s\n", outPath)
	return nil
}

func resizeImage(src image.Image, targetW, targetH int) image.Image {
	bounds := src.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	if targetW == 0 {
		targetW = (origW * targetH) / origH
	}
	if targetH == 0 {
		targetH = (origH * targetW) / origW
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	
	draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
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