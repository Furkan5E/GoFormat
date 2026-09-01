package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"goformat/converter"
	"goformat/format"
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
	fmt.Printf("Scanning directory: %s\n", dirPath)
	jobs := make(chan string, 100)
	var wg sync.WaitGroup

	numWorkers := runtime.NumCPU() //optimal number of workers based on hardware
	fmt.Printf("Initialising worker pool with %d concurrent threads...\n", numWorkers)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//worker constantly pulls from channel until it is closed
			for path := range jobs {
				converter.ProcessImage(path, outDir, targetFormat, quality)
			}
		}()
	}

	//walk directory and push valid files into jobs
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && path != dirPath {
			return filepath.SkipDir
		}

		ext := strings.ToLower(filepath.Ext(path))
		if format.IsSupported(ext) {
			jobs <- path //send the file path into the queue
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
	}

	close(jobs)
	wg.Wait()
	fmt.Println("Batch processing complete.")
}