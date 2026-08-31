package converter

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

func ProcessImage(inputPath string, outFormat string, quality int) {
	file, err := os.Open(inputPath)
	if err != nil {
		fmt.Printf("Failed to open file: %v\n", err)
		return
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		fmt.Printf("Failed to decode image: %v\n", err)
		return
	}

	fmt.Printf("loaded %s image. dimension: %v\n", format, img.Bounds())
	fmt.Printf("ready to convert to %s at %d quality.\n", outFormat, quality)
}