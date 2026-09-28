package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"goformat/batch"
	"goformat/converter"
	"goformat/format"
)

//exit codes
const (
	exitOK          = 0
	exitFailure     = 1
	exitUsage       = 2
	exitInterrupted = 130
)

func main() {
	os.Exit(run(os.Args[1:]))
}

//run is separate from main so deferred calls finish before os.Exit
func run(args []string) int {
	flags := flag.NewFlagSet("goformat", flag.ContinueOnError)
	inputPath := flags.String("i", "", "Path to the input image or directory (required)")
	outDir := flags.String("o", "output", "Path to the output directory")
	targetFormat := flags.String("f", "jpeg", "Target format: jpeg, png, webp, tiff, bmp, gif, ico, avif")
	quality := flags.Int("q", 85, "Compression quality for jpeg/webp/avif (1-100)")
	recursive := flags.Bool("r", false, "Process subdirectories recursively")
	width := flags.Int("width", 0, "Target width in pixels (0 to keep original)")
	height := flags.Int("height", 0, "Target height in pixels (0 to keep original)")
	pixelart := flags.Bool("pixel", false, "Use nearest neighbour scaling to preserve pixel edges")
	workers := flags.Int("workers", runtime.NumCPU(), "Number of images to convert at once in batch mode")
	if err := flags.Parse(args); err != nil {
		//the flag package has already printed the error or usage
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitUsage
	}

	if *inputPath == "" {
		fmt.Println("Error: Input path is required. Use -i <path>")
		return exitUsage
	}

	if *width < 0 || *height < 0 {
		fmt.Println("Error: -width and -height cannot be negative")
		return exitUsage
	}

	if *quality < 1 || *quality > 100 {
		fmt.Println("Error: -q must be between 1 and 100")
		return exitUsage
	}

	if *workers < 1 {
		fmt.Println("Error: -workers must be at least 1")
		return exitUsage
	}

	//checked up front so a batch doesn't fail the same way once per file
	if _, err := format.GetEncoder(strings.ToLower(*targetFormat)); err != nil {
		fmt.Printf("Error: %v\n", err)
		return exitUsage
	}

	//create output directory if does not exist
	err := os.MkdirAll(*outDir, os.ModePerm)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return exitFailure
	}

	info, err := os.Stat(*inputPath)
	if err != nil {
		fmt.Printf("Error accessing input path: %v\n", err)
		return exitFailure
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if info.IsDir() {
		err = batch.ProcessDirectory(ctx, *inputPath, *outDir, batch.Options{
			Format:    *targetFormat,
			Quality:   *quality,
			Width:     *width,
			Height:    *height,
			Pixelart:  *pixelart,
			Recursive: *recursive,
			Workers:   *workers,
		})
	} else {
		var res converter.Result
		res, err = converter.ProcessImage(ctx, *inputPath, *outDir, *targetFormat, *quality, *width, *height, *pixelart)
		if err != nil {
			fmt.Printf("Error processing file: %v\n", err)
		} else {
			fmt.Printf("Saved converted file as: %s\n", res.OutPath)
			for _, msg := range res.Warnings {
				fmt.Printf("Warning: %s\n", msg)
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