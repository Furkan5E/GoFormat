package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/Furkan5E/GoFormat/v2/batch"
	"github.com/Furkan5E/GoFormat/v2/converter"
	"github.com/Furkan5E/GoFormat/v2/format"
)

// exit codes
const (
	exitOK          = 0
	exitFailure     = 1
	exitUsage       = 2
	exitInterrupted = 130
)

// version is set by release builds with -ldflags "-X main.version=..."
var version string

// buildVersion falls back to what the Go toolchain recorded, which covers go install
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is separate from main so deferred calls finish before os.Exit
func run(args []string) int {
	flags := flag.NewFlagSet("goformat", flag.ContinueOnError)
	inputPath := flags.String("i", "", "Path to the input image or directory (required)")
	outDir := flags.String("o", "output", "Path to the output directory")
	targetFormat := flags.String("f", "jpeg", "Target format: jpeg, png, webp, tiff, bmp, gif, ico, avif")
	quality := flags.Int("q", 85, "Compression quality for jpeg/webp/avif (1-100)")
	recursive := flags.Bool("r", false, "Process subdirectories recursively")
	width := flags.Int("width", 0, "Target width in pixels (0 to keep original)")
	height := flags.Int("height", 0, "Target height in pixels (0 to keep original)")
	fitName := flags.String("fit", "stretch", "How an image is fitted when both -width and -height are set: stretch, contain or cover")
	pixelart := flags.Bool("pixel", false, "Use nearest neighbour scaling to preserve pixel edges")
	lossless := flags.Bool("lossless", false, "Write lossless webp, keeping every pixel exactly (-q is ignored)")
	strip := flags.Bool("strip", false, "Leave out EXIF data, the colour profile and the original date")
	skipExisting := flags.Bool("skip-existing", false, "Leave files that already exist in the output directory untouched")
	dryRun := flags.Bool("dry-run", false, "Show what would be converted without writing anything")
	maxMegapixels := flags.Int("max-megapixels", converter.DefaultMaxPixels/1_000_000, "Largest input image accepted, in megapixels (0 for no limit)")
	workers := flags.Int("workers", runtime.NumCPU(), "Number of images to convert at once in batch mode")
	verbose := flags.Bool("v", false, "Print a detailed line for every file")
	quiet := flags.Bool("quiet", false, "Print nothing but errors and warnings")
	jsonReport := flags.Bool("json", false, "Print a JSON report of the run instead of the usual output")
	showVersion := flags.Bool("version", false, "Print the version and exit")
	if err := flags.Parse(args); err != nil {
		//the flag package has already printed the error or usage
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	if *showVersion {
		fmt.Printf("goformat %s\n", buildVersion())
		return exitOK
	}

	if *inputPath == "" {
		fmt.Fprintln(os.Stderr, "Error: Input path is required. Use -i <path>")
		return exitUsage
	}

	if *width < 0 || *height < 0 {
		fmt.Fprintln(os.Stderr, "Error: -width and -height cannot be negative")
		return exitUsage
	}

	fit, err := converter.ParseFit(*fitName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return exitUsage
	}
	if fit != converter.FitStretch && (*width == 0 || *height == 0) {
		fmt.Fprintf(os.Stderr, "Error: -fit %s needs both -width and -height\n", fit)
		return exitUsage
	}

	if *quality < 1 || *quality > 100 {
		fmt.Fprintln(os.Stderr, "Error: -q must be between 1 and 100")
		return exitUsage
	}

	//png, tiff, bmp, gif and ico are lossless anyway, so the flag is harmless there
	if f := strings.ToLower(*targetFormat); *lossless && (f == "jpeg" || f == "jpg" || f == "avif") {
		fmt.Fprintf(os.Stderr, "Error: -lossless is not available for %s, use webp or png\n", f)
		return exitUsage
	}

	if *maxMegapixels < 0 {
		fmt.Fprintln(os.Stderr, "Error: -max-megapixels cannot be negative")
		return exitUsage
	}

	if *jsonReport && *verbose {
		fmt.Fprintln(os.Stderr, "Error: -json and -v cannot be used together")
		return exitUsage
	}

	if *verbose && *quiet {
		fmt.Fprintln(os.Stderr, "Error: -v and -quiet cannot be used together")
		return exitUsage
	}

	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "Error: -workers must be at least 1")
		return exitUsage
	}

	//checked up front so a batch doesn't fail the same way once per file
	if _, err := format.GetEncoder(strings.ToLower(*targetFormat)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return exitUsage
	}

	//create output directory if does not exist, except on a dry run which leaves the disk untouched
	if !*dryRun {
		if err := os.MkdirAll(*outDir, os.ModePerm); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			return exitFailure
		}
	}

	info, err := os.Stat(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error accessing input path: %v\n", err)
		return exitFailure
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	opts := converter.Options{
		Format:        *targetFormat,
		Quality:       *quality,
		Lossless:      *lossless,
		Width:         *width,
		Height:        *height,
		Fit:           fit,
		Pixelart:      *pixelart,
		StripMetadata: *strip,
		SkipExisting:  *skipExisting,
		DryRun:        *dryRun,
		MaxPixels:     int64(*maxMegapixels) * 1_000_000,
	}

	if info.IsDir() {
		batchOpts := batch.Options{
			Options:   opts,
			Recursive: *recursive,
			Workers:   *workers,
			Verbose:   *verbose,
			Quiet:     *quiet,
		}
		if *jsonReport {
			batchOpts.JSON = os.Stdout
		}
		err = batch.ProcessDirectory(ctx, *inputPath, *outDir, batchOpts)
	} else {
		var res converter.Result
		start := time.Now()
		res, err = converter.ProcessImage(ctx, *inputPath, *outDir, opts)
		if *jsonReport {
			//a single file gets the same report as a directory, with one entry
			report := batch.Report{
				Input:     *inputPath,
				Output:    *outDir,
				Format:    strings.ToLower(*targetFormat),
				DryRun:    *dryRun,
				Cancelled: ctx.Err() != nil,
			}
			report.Add(batch.NewFileReport(*inputPath, converter.OutputPath(*inputPath, *outDir, *targetFormat), res, err))
			report.ElapsedMs = time.Since(start).Milliseconds()
			if writeErr := report.Write(os.Stdout); writeErr != nil {
				fmt.Fprintf(os.Stderr, "Error writing the report: %v\n", writeErr)
			}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing file: %v\n", err)
		} else {
			switch {
			case *quiet:
			case *verbose:
				fmt.Println(res.Verbose())
			default:
				fmt.Println(res.Summary())
			}
			for _, msg := range res.Warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
			}
		}
	}

	switch {
	case ctx.Err() != nil:
		return exitInterrupted
	case err != nil:
		return exitFailure
	}
	return exitOK
}
