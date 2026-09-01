package converter

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"goformat/format"
)

func ProcessImage(inputPath string, outDir string, outFormat string, quality int) error {
	outFormat = strings.ToLower(outFormat)
	enc, err := format.GetEncoder(outFormat)
	if err != nil {
		return err
	}

	img, err := loadImage(inputPath)
	if err != nil {
		return fmt.Errorf("failed to load %s: %v", inputPath, err)
	}

	outPath := generateOutputPath(inputPath, outDir, outFormat)

	err = saveImage(img, outPath, enc, quality)
	if err != nil {
		return fmt.Errorf("failed to save %s: %v", outPath, err)
	}

	fmt.Printf("Success! Saved converted file as: %s\n", outPath)
	return nil
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