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

func ProcessDirectory(dirPath string, outDir string, targetFormat string, quality int) {
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