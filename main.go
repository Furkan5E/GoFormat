package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"goformat/batch"
	"goformat/converter"
)

func main() {
	inputPath := flag.String("i", "", "Path to the input image or directory (required)")
	outDir := flag.String("o", "output", "Path to the output directory")
	targetFormat := flag.String("f", "jpeg", "Target format: jpeg, png, webp")
	quality := flag.Int("q", 85, "Compression quality for jpeg/webp (1-100)")
	recursive := flag.Bool("r", false, "Process subdirectories recursively")
	width := flag.Int("w", 0, "Target width in pixels (0 to keep original)")
	height := flag.Int("h", 0, "Target height in pixels (0 to keep original)")
	pixelart := flag.Bool("pixel", false, "Use nearest neighbour scaling to preserve pixel edges")
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if info.IsDir() {
		batch.ProcessDirectory(ctx, *inputPath, *outDir, *targetFormat, *quality, *recursive, *width, *height, *pixelart)
	} else {
		err := converter.ProcessImage(ctx, *inputPath, *outDir, *targetFormat, *quality, *width, *height, *pixelart)
		if err != nil {
			fmt.Printf("Error processing file: %v\n", err)
		}
	}
}