package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"goformat/converter"
)

func main() {
	inputPath := flag.String("i", "", "Path to the input image or directory (required)")
	outDir := flag.String("o", "output", "Path to the output directory")
	targetFormat := flag.String("f", "jpeg", "Target format: jpeg, png, webp")
	quality := flag.Int("q", 85, "Compression quality for jpeg/webp (1-100)")
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
		processDirectory(*inputPath, *outDir, *targetFormat, *quality)
	} else {
		converter.ProcessImage(*inputPath, *outDir, *targetFormat, *quality)
	}
}

func processDirectory(dirPath string, outDir string, targetFormat string, quality int) {
	var wg sync.WaitGroup

	fmt.Printf("Scanning directory: %s\n", dirPath)

	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && path != dirPath {
			return filepath.SkipDir
		}

		//process supported image files
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			wg.Add(1)

			//goroutine for each file
			go func(p string) {
				defer wg.Done()
				converter.ProcessImage(p, outDir, targetFormat, quality)
			}(path)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
	}

	wg.Wait()
	fmt.Println("Batch processing complete.")
}