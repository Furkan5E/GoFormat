# GoFormat

![Go Version](https://img.shields.io/github/go-mod/go-version/Furkan5E/GoFormat?logo=go&color=00ADD8)
![Platform](https://img.shields.io/badge/Platform-windows%20%7C%20macos%20%7C%20linux-lightgrey)
![Licence](https://img.shields.io/github/license/Furkan5E/GoFormat?label=Licence&color=blue)
[![Build Status](https://github.com/Furkan5E/GoFormat/actions/workflows/build.yaml/badge.svg)](https://github.com/Furkan5E/GoFormat/actions/workflows/build.yaml)

A high performance, concurrent command-line image processing utility written in Go. GoFormat is designed for bulk asset conversion, resizing and standardisation.

[![Download Latest Release](https://img.shields.io/github/v/release/Furkan5E/GoFormat?style=for-the-badge&label=Download&color=success&logo=github)](https://github.com/Furkan5E/GoFormat/releases/latest)

## Features
* **Format Conversion:** Convert between nine image formats, including AVIF, ICO and iPhone HEIC photos.
* **Batch Processing:** Process entire directories concurrently using multi core worker pools, with a progress bar and a summary report.
* **Image Resizing:** Scale images up or down to specific dimensions.
* **Compression Control:** Adjust the output quality of JPEG, WebP and AVIF to optimise file sizes.
* **Pixel Art Support:** Upscale pixel art and low-resolution graphics using nearest-neighbour scaling to preserve edges without blurring.
* **Animated GIFs:** Converting GIF to GIF keeps every frame, frame delays and the loop count, including when resizing.
* **Transparency:** Transparent areas are preserved when converting to GIF.

## Supported Formats

| Format | Extensions | Read | Write |
|--------|------------|------|-------|
| JPEG | `.jpeg`, `.jpg` | Yes | Yes |
| PNG | `.png` | Yes | Yes |
| WebP | `.webp` | Yes | Yes |
| TIFF | `.tiff`, `.tif` | Yes | Yes |
| BMP | `.bmp` | Yes | Yes |
| GIF | `.gif` | Yes | Yes |
| ICO | `.ico` | Yes | Yes |
| AVIF | `.avif` | Yes | Yes |
| HEIC | `.heic`, `.heif` | Yes | No |

## Installation

### Download
Download the binary for your platform from the [latest release](https://github.com/Furkan5E/GoFormat/releases/latest):

| Platform | File |
|----------|------|
| Windows (64-bit) | `goformat-windows-amd64.exe` |
| Linux (64-bit) | `goformat-linux-amd64` |
| macOS (Apple Silicon) | `goformat-macos-arm64` |

Each release includes a `checksums.txt` for verifying downloads. On Linux and macOS, make the file executable first:
```bash
chmod +x goformat-linux-amd64
```

### Build from Source
Requires Go 1.27 or newer.
```bash
git clone https://github.com/Furkan5E/GoFormat.git
cd GoFormat
```
Compile the tool into an executable:
```bash
# Windows
go build -ldflags="-s -w" -o goformat.exe .

# Linux / macOS
go build -ldflags="-s -w" -o goformat .
```
Or run it without compiling by replacing `.\goformat.exe` with `go run .` in the examples below.

## Usage
The examples use Windows syntax. On Linux and macOS, use `./goformat` instead of `.\goformat.exe`.

Convert a single image:
```bash
.\goformat.exe -i source.jpg -o final_images -f png
```
Batch process a directory, including subdirectories:
```bash
.\goformat.exe -i pictures -r -f webp -q 80 -width 1920
```
Upscale 2D assets:
```bash
.\goformat.exe -i sprites -o assets -f png -width 1024 -pixel
```

### Resizing
Set only `-width` or only `-height` to keep the original aspect ratio. Setting both resizes to exactly that size, which stretches the image if the proportions differ:
```bash
.\goformat.exe -i pictures -f tiff -width 1920 -height 1080
```

### Batch Processing
* The output directory mirrors the input folder structure, and folders without images are not copied.
* If the output directory is inside the input directory, it is skipped so earlier results are not converted again.
* Files in the same folder that share a name (`a.png` and `a.jpg`) would produce the same output file, so each keeps its original extension in the output name (`a_png.webp` and `a_jpg.webp`).
* A progress bar with an estimated time remaining is shown in the terminal. When output is redirected to a file or another program, one line per file is printed instead.
* The final report lists skipped unsupported files, failed files and any warnings.

### Animated GIFs
Converting an animated GIF to GIF keeps all frames. Converting it to any other format keeps only the first frame, as those formats do not support animation.

## Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-i` | Path to the input image or directory |`Required`|
| `-o` | Path to the output directory | `output` |
| `-f` | Target format (`jpeg`, `png`, `webp`, `tiff`, `bmp`, `gif`, `ico`, `avif`) | `jpeg` |
| `-q` | Compression quality for JPEG, WebP and AVIF (1 to 100) | `85` |
| `-width` | Target width in pixels (0 to keep original) | `0` |
| `-height` | Target height in pixels (0 to keep original) | `0` |
| `-pixel` | Use nearest neighbour scaling to preserve pixel edges | `false` |
| `-r` | Process subdirectories recursively | `false` |
| `-workers` | Number of images to convert at once in batch mode | CPU cores |
| `-h` | Show help | |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All files converted successfully |
| `1` | One or more files failed, or the input/output path could not be used |
| `2` | Invalid usage, such as a missing `-i` |
| `130` | Interrupted with Ctrl+C before finishing |

## Running Tests
```bash
go test ./...
```
