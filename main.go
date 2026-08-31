package main

import (
	"flag"
	"fmt"
	"os"
	"goformat/converter" 
)

func main() {
	inputPath := flag.String("i", "", "Path to the input image or directory (required)")
	outFormat := flag.String("f", "jpeg", "Target format (jpeg, png, webp, tiff)")
	quality := flag.Int("q", 85, "Compression quality for jpeg/webp (1-100)")

	flag.Parse()

	if *inputPath == "" {
		fmt.Println("Error: Input path is required.")
		fmt.Println("Usage: go run main.go -i <input_file> [-f <format>] [-q <quality>]")
		os.Exit(1)
	}


	converter.ProcessImage(*inputPath, *outFormat, *quality)
}