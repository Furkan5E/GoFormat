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

type Job struct {
	InputPath string
	OutputDir string
}

func ProcessDirectory(dirPath string, outDir string, targetFormat string, quality int, recursive bool, width int, height int) {
	fmt.Printf("Scanning directory: %s\n", dirPath)
	jobs := make(chan Job, 100)
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
			for job := range jobs {
				err := converter.ProcessImage(job.InputPath, job.OutputDir, targetFormat, quality, width, height)
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

		if d.IsDir() {
			//skip subdirectories if recursive flag is false
			if !recursive && path != dirPath {
				return filepath.SkipDir
			}
			
			//recreate target directory structure inside output folder
			relPath, relErr := filepath.Rel(dirPath, path)
			if relErr == nil {
				targetDir := filepath.Join(outDir, relPath)
				os.MkdirAll(targetDir, os.ModePerm)
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if format.IsSupported(ext) {
			//calculate output directory for file
			relPath, _ := filepath.Rel(dirPath, filepath.Dir(path))
			targetDir := filepath.Join(outDir, relPath)
			
			jobs <- Job{InputPath: path, OutputDir: targetDir}
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