package main

import (
	"flag"
	"fmt"
	"os"

	"goformat/batch"
	"goformat/converter"
)

func main() {
	inputPath := flag.String("i", "", "Path to the input image or directory (required)")
	outDir := flag.String("o", "output", "Path to the output directory")
	targetFormat := flag.String("f", "jpeg", "Target format: jpeg, png, webp")
	quality := flag.Int("q", 85, "Compression quality for jpeg/webp (1-100)")
	recursive := flag.Bool("r", false, "Process subdirectories recursively")
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("Error: Input path is required. Use -i <path>")
		return
	}

	//create output directory if does not exist
	err := os.MkdirAll(*outDir, os.ModePerm)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	info, err := os.Stat(*inputPath)
	if err != nil {
		fmt.Printf("Error accessing input path: %v\n", err)
		return
	}

	if info.IsDir() {
		batch.ProcessDirectory(*inputPath, *outDir, *targetFormat, *quality, *recursive)
	} else {
		err := converter.ProcessImage(*inputPath, *outDir, *targetFormat, *quality)
		if err != nil {
			fmt.Printf("Error processing file: %v\n", err)
		}
	}
}