# GoFormat

![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/Platform-windows%20%7C%20macos%20%7C%20linux-lightgrey)
![Licence](https://img.shields.io/badge/License-MIT-blue)
[![Build Status](https://github.com/Furkan5E/GoFormat/actions/workflows/build.yaml/badge.svg)](https://github.com/Furkan5E/GoFormat/actions/workflows/build.yaml)

A high performance, concurrent command-line image processing utility written in Go. GoFormat is designed for bulk asset conversion, resizing and standardisation.

[![Download Latest Release](https://img.shields.io/github/v/release/Furkan5E/GoFormat?style=for-the-badge&label=Download%20.exe&color=success)](https://github.com/Furkan5E/GoFormat/releases/latest)

## Features
* **Format Conversion:** Convert images between six file formats.
* **Batch Processing:** Process entire directories concurrently using multi core worker pools.
* **Image Resizing:** Scale images up or down to specific dimensions.
* **Compression Control:** Adjust the output quality of applicable formats to optimise file sizes.
* **Pixel Art Support:** Upscale pixel art and low-resolution graphics using nearest-neighbour scaling to preserve edges without blurring.

## Supported Formats
* `.jpeg`
* `.png`
* `.webp`
* `.tiff`
* `.bmp`
* `.gif`

## Installation
Clone the repository
```bash
git clone https://github.com/Furkan5E/GoFormat.git
cd GoFormat
```
### Usage
Convert a single image:
```bash
go run main.go -i source.png -f jpg
```
Batch process a directory:
```bash
go run main.go -i pictures -f tiff -w 1920 -h 1080
```

## Compilation
Compile the tool into a executable:
```bash
go build -ldflags="-s -w" -o goformat.exe main.go
```
### Usage
Convert a single image:
```bash
.\goformat.exe -i source.jpg -o final_images -f png
```
Batch process a directory:
```bash
.\goformat.exe -i pictures -f webp -q 80 -w 1920
```
Upscale 2D assets:
```bash
.\goformat.exe -i sprites -o assets -f png -w 1024 -pixel
```
## Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-i` | Path to the input image or directory |`Required`|
| `-o` | Path to the output directory | `output` |
| `-f` | Target format (`jpeg`, `png`, `webp`, `tiff`, `bmp`, `gif`) | `jpeg` |
| `-q` | Compression quality for applicable formats (1 to 100) | `85` |
| `-w` | Target width in pixels (0 to keep original) | `0` |
| `-h` | Target height in pixels (0 to keep original) | `0` |
| `-pixel` | Use nearest neighbour scaling to preserve pixel edges | `false` |
| `-r` | Process subdirectories recursively | `false` |
