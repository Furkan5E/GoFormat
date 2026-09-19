package batch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"goformat/converter"
	"goformat/format"
)

type Job struct {
	InputPath string
	OutputDir string
}

type Options struct {
	Format    string
	Quality   int
	Width     int
	Height    int
	Pixelart  bool
	Recursive bool
	Workers   int
}

func ProcessDirectory(ctx context.Context, dirPath string, outDir string, opts Options) error {
	fmt.Printf("Scanning directory: %s\n", dirPath)

	//collect everything first so the progress bar knows the total
	jobList, skipped, err := collectJobs(ctx, dirPath, outDir, opts.Recursive)

	var walkErr error
	if err != nil && err != context.Canceled {
		fmt.Printf("Error reading directory: %v\n", err)
		walkErr = err
	}

	if ctx.Err() == nil {
		fmt.Printf("Found %d images to convert\n", len(jobList))
	}

	var errMu sync.Mutex
	var failedJobs []string

	if len(jobList) > 0 && ctx.Err() == nil {
		numWorkers := min(opts.Workers, len(jobList))
		fmt.Printf("Initialising worker pool with %d concurrent threads...\n", numWorkers)

		bar := newProgress(len(jobList))
		jobs := make(chan Job, 100)
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				//worker constantly pulls from channel until it is closed
				for job := range jobs {
					if ctx.Err() != nil {
						return //exit goroutine
					}

					outPath, err := convert(ctx, job, opts)
					if err != nil {
						errMu.Lock()
						failedJobs = append(failedJobs, err.Error())
						errMu.Unlock()
					}
					bar.finish(outPath, err)
				}
			}()
		}

	feed:
		for _, job := range jobList {
			select {
			case jobs <- job:
			case <-ctx.Done():
				break feed
			}
		}

		close(jobs)
		wg.Wait()
		bar.close()
	}

	fmt.Println("\n=== Batch Processing Report ===")
	if ctx.Err() != nil {
		fmt.Println("Process was cancelled. Partial results saved.")
	}
	if skipped > 0 {
		fmt.Printf("Skipped %d unsupported files.\n", skipped)
	}
	if len(failedJobs) > 0 {
		fmt.Printf("Completed with %d errors:\n", len(failedJobs))
		for _, errMsg := range failedJobs {
			fmt.Printf("  x %s\n", errMsg)
		}
	} else if ctx.Err() == nil && walkErr == nil {
		fmt.Println("All files processed successfully with zero errors.")
	}

	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case walkErr != nil:
		return walkErr
	case len(failedJobs) > 0:
		return fmt.Errorf("%d files failed", len(failedJobs))
	}
	return nil
}

//walk dirPath and return a job per supported image, plus how many other files were skipped
func collectJobs(ctx context.Context, dirPath, outDir string, recursive bool) ([]Job, int, error) {
	var jobList []Job
	skipped := 0

	//output folder may sit inside the input folder (e.g. -i . -o output)
	outInfo, _ := os.Stat(outDir)

	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if d.IsDir() {
			//skip subdirectories if recursive flag is false
			if !recursive && path != dirPath {
				return filepath.SkipDir
			}

			//never walk into our own output, or earlier results get converted again
			if path != dirPath && outInfo != nil {
				if info, statErr := os.Stat(path); statErr == nil && os.SameFile(info, outInfo) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !format.IsSupported(ext) {
			skipped++
			return nil
		}

		//mirror the input folder structure inside the output folder
		relPath, _ := filepath.Rel(dirPath, filepath.Dir(path))
		jobList = append(jobList, Job{InputPath: path, OutputDir: filepath.Join(outDir, relPath)})
		return nil
	})

	return jobList, skipped, err
}

func convert(ctx context.Context, job Job, opts Options) (string, error) {
	//created on demand so folders without images don't leave empty copies behind
	if err := os.MkdirAll(job.OutputDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create %s: %v", job.OutputDir, err)
	}
	return converter.ProcessImage(ctx, job.InputPath, job.OutputDir, opts.Format, opts.Quality, opts.Width, opts.Height, opts.Pixelart)
}
