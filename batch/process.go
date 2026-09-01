package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"goformat/converter"
	"goformat/format"
)

func ProcessDirectory(dirPath string, outDir string, targetFormat string, quality int, recursive bool) {
	fmt.Printf("Scanning directory: %s\n", dirPath)
	jobs := make(chan string, 100)
	var wg sync.WaitGroup

	var errMu sync.Mutex
	var failedJobs []string

	numWorkers := runtime.NumCPU() //optimal number of workers based on hardware
	fmt.Printf("Initialising worker pool with %d concurrent threads...\n", numWorkers)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//worker constantly pulls from channel until it is closed
			for path := range jobs {
				err := converter.ProcessImage(path, outDir, targetFormat, quality)
				if err != nil {
					errMu.Lock()
					failedJobs = append(failedJobs, err.Error())
					errMu.Unlock()
				}
			}
		}()
	}

	//walk directory and push valid files into jobs
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		//skip subdirectories if recursive flag is false
		if !recursive && d.IsDir() && path != dirPath {
			return filepath.SkipDir
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !d.IsDir() && format.IsSupported(ext) {
			jobs <- path //send the file path into the queue
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
	}

	close(jobs)
	wg.Wait()

	fmt.Println("\n=== Batch Processing Report ===")
	if len(failedJobs) > 0 {
		fmt.Printf("Completed with %d errors:\n", len(failedJobs))
		for _, errMsg := range failedJobs {
			fmt.Printf("  x %s\n", errMsg)
		}
	} else {
		fmt.Println("All files processed successfully with zero errors.")
	}
}