package batch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Furkan5E/GoFormat/v2/converter"
	"github.com/Furkan5E/GoFormat/v2/format"
)

type Job struct {
	InputPath string
	OutputDir string
	//output file name without its extension
	OutputName string
}

type Options struct {
	converter.Options
	Recursive bool
	Workers   int
	//print a detailed line per file instead of the progress bar
	Verbose bool
	//print nothing but errors and warnings
	Quiet bool
	//when set, a JSON report of the run is written here and nothing else is printed
	JSON io.Writer
}

// say prints progress information, which Quiet turns off
func (o Options) say(format string, args ...any) {
	if !o.Quiet {
		fmt.Printf(format, args...)
	}
}

func ProcessDirectory(ctx context.Context, dirPath string, outDir string, opts Options) error {
	start := time.Now()
	//the report replaces everything meant for people, so that what is printed can be parsed
	if opts.JSON != nil {
		opts.Quiet, opts.Verbose = true, false
	}
	opts.say("Scanning directory: %s\n", dirPath)

	//small images still run on every worker, large ones wait for each other
	if opts.Memory == nil {
		opts.Memory = converter.NewBudget(converter.DefaultMaxPixels)
	}

	//collect everything first so the progress bar knows the total
	jobList, skipped, err := collectJobs(ctx, dirPath, outDir, opts.Recursive)

	var walkErr error
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
		walkErr = err
	}

	if ctx.Err() == nil {
		opts.say("Found %d images to convert\n", len(jobList))
	}

	var errMu sync.Mutex
	var failedJobs []string
	var warnings []string
	converted, replaced, existing := 0, 0, 0
	//one entry per job, in the order the files were found rather than the order they happened to finish in
	files := make([]*FileReport, len(jobList))

	if len(jobList) > 0 && ctx.Err() == nil {
		numWorkers := min(opts.Workers, len(jobList))
		if opts.DryRun {
			//nothing is converted, and a single worker lists the files in order
			numWorkers = 1
		} else {
			opts.say("Initialising worker pool with %d concurrent threads...\n", numWorkers)
		}

		bar := newProgress(len(jobList), opts)
		jobs := make(chan int, 100)
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				//worker constantly pulls from channel until it is closed
				for i := range jobs {
					if ctx.Err() != nil {
						return //exit goroutine
					}

					job := jobList[i]
					res, err := convert(ctx, job, opts)
					file := NewFileReport(job.InputPath, job.outputPath(opts.Format), res, err)
					errMu.Lock()
					files[i] = &file
					switch {
					case err != nil:
						failedJobs = append(failedJobs, err.Error())
					case res.Skipped:
						existing++
					default:
						converted++
						if res.Replaced {
							replaced++
						}
					}
					warnings = append(warnings, res.Warnings...)
					errMu.Unlock()
					bar.finish(res, err)
				}
			}()
		}

	feed:
		for i := range jobList {
			select {
			case jobs <- i:
			case <-ctx.Done():
				break feed
			}
		}

		close(jobs)
		wg.Wait()
		bar.close()
	}

	if opts.DryRun {
		opts.say("\n=== Dry Run Report ===\n")
		opts.say("Nothing was written.\n")
		opts.say("Would convert %d files.\n", converted)
	} else {
		opts.say("\n=== Batch Processing Report ===\n")
	}
	if ctx.Err() != nil {
		opts.say("Process was cancelled. Partial results saved.\n")
	}
	if skipped > 0 {
		opts.say("Skipped %d unsupported files.\n", skipped)
	}
	if replaced > 0 {
		if opts.DryRun {
			opts.say("Would replace %d files that already exist.\n", replaced)
		} else {
			opts.say("Replaced %d files that already existed.\n", replaced)
		}
	}
	if existing > 0 {
		if opts.DryRun {
			opts.say("Would skip %d files that already exist.\n", existing)
		} else {
			opts.say("Skipped %d files that already existed.\n", existing)
		}
	}
	if opts.JSON != nil {
		report := Report{
			Input:     dirPath,
			Output:    outDir,
			Format:    strings.ToLower(opts.Format),
			DryRun:    opts.DryRun,
			Cancelled: ctx.Err() != nil,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
		if walkErr != nil {
			report.Error = walkErr.Error()
		}
		//files a cancelled run never reached have no entry
		for _, file := range files {
			if file != nil {
				report.Add(*file)
			}
		}
		report.Summary.Unsupported = skipped
		if err := report.Write(opts.JSON); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing the report: %v\n", err)
		}
	} else if len(warnings) > 0 {
		//problems go to stderr so they stay visible when the report is redirected
		fmt.Fprintf(os.Stderr, "%d warnings:\n", len(warnings))
		for _, msg := range warnings {
			fmt.Fprintf(os.Stderr, "  ! %s\n", msg)
		}
	}
	if opts.JSON != nil {
		//the failures are listed in the report
	} else if len(failedJobs) > 0 {
		fmt.Fprintf(os.Stderr, "Completed with %d errors:\n", len(failedJobs))
		for _, errMsg := range failedJobs {
			fmt.Fprintf(os.Stderr, "  x %s\n", errMsg)
		}
	} else if ctx.Err() == nil && walkErr == nil && !opts.DryRun {
		opts.say("All files processed successfully with zero errors.\n")
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

// walk dirPath and return a job per supported image, plus how many other files were skipped
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
		jobList = append(jobList, Job{
			InputPath:  path,
			OutputDir:  filepath.Join(outDir, relPath),
			OutputName: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		})
		return nil
	})

	resolveCollisions(jobList)
	return jobList, skipped, err
}

// inputs sharing a base name in one folder (a.png and a.jpg) would otherwise overwrite each other's output
// each of them gets its source extension added to the name instead (a_png, a_jpg)
func resolveCollisions(jobs []Job) {
	//compared in lower case as Windows and macOS file names are case-insensitive
	key := func(j Job) string {
		return strings.ToLower(filepath.Join(j.OutputDir, j.OutputName))
	}

	count := make(map[string]int)
	for _, j := range jobs {
		count[key(j)]++
	}
	taken := make(map[string]bool)
	for _, j := range jobs {
		if count[key(j)] == 1 {
			taken[key(j)] = true
		}
	}

	for i := range jobs {
		j := &jobs[i]
		if count[key(*j)] == 1 {
			continue
		}
		base := j.OutputName + "_" + strings.TrimPrefix(filepath.Ext(j.InputPath), ".")
		j.OutputName = base
		//the new name can itself be in use (a real a_png.gif, or a.png next to a.PNG), so number it
		for n := 2; taken[key(*j)]; n++ {
			j.OutputName = fmt.Sprintf("%s_%d", base, n)
		}
		taken[key(*j)] = true
	}
}

func convert(ctx context.Context, job Job, opts Options) (converter.Result, error) {
	//created on demand so folders without images don't leave empty copies behind
	if opts.DryRun {
		//a dry run leaves the disk untouched
	} else if err := os.MkdirAll(job.OutputDir, os.ModePerm); err != nil {
		return converter.Result{}, fmt.Errorf("failed to create %s: %w", job.OutputDir, err)
	}
	return converter.ProcessImageTo(ctx, job.InputPath, job.outputPath(opts.Format), opts.Options)
}

func (j Job) outputPath(format string) string {
	return filepath.Join(j.OutputDir, j.OutputName+"."+strings.ToLower(format))
}
