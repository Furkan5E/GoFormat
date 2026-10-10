package format

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// Metadata is what an image file carries besides its pixels
type Metadata struct {
	//EXIF block, starting at its TIFF header: camera, lens, exposure, date and GPS position
	EXIF []byte
	//ICC colour profile, which says what the colour values mean (sRGB, Display P3 and so on)
	ICC []byte
}

func (m Metadata) Empty() bool {
	return len(m.EXIF) == 0 && len(m.ICC) == 0
}

// CarriesMetadata reports whether EmbedMetadata can write into the named output format
func CarriesMetadata(name string) bool {
	switch name {
	case "jpeg", "jpg", "png", "webp":
		return true
	}
	return false
}

// profiles and EXIF blocks are a few kB to a few MB, anything far larger is not worth trusting
const maxMetadataSize = 16 << 20

// ReadMetadata pulls the EXIF block and ICC profile out of a JPEG, PNG or WebP file
// the format is recognised by content, and anything else or anything malformed gives empty metadata
func ReadMetadata(data []byte) Metadata {
	var m Metadata
	switch {
	case isJPEG(data):
		m = readJPEGMetadata(data)
	case isPNG(data):
		m = readPNGMetadata(data)
	case isWebP(data):
		m = readWebPMetadata(data)
	}

	if !validEXIF(m.EXIF) {
		m.EXIF = nil
	}
	if len(m.ICC) > maxMetadataSize {
		m.ICC = nil
	}
	return m
}

// ReadMetadataFile is ReadMetadata for a file on disk
// only the first few bytes are read unless the file is one of the formats that can be searched
func ReadMetadataFile(path string) Metadata {
	file, err := os.Open(path)
	if err != nil {
		return Metadata{}
	}
	defer file.Close()

	header := make([]byte, webpHeaderSize)
	n, _ := io.ReadFull(file, header)
	header = header[:n]
	if !isJPEG(header) && !isPNG(header) && !isWebP(header) {
		return Metadata{}
	}

	rest, err := io.ReadAll(file)
	if err != nil {
		return Metadata{}
	}
	return ReadMetadata(append(header, rest...))
}

func isJPEG(data []byte) bool { return bytes.HasPrefix(data, []byte{0xFF, 0xD8}) }
func isPNG(data []byte) bool  { return bytes.HasPrefix(data, pngSignature) }
func isWebP(data []byte) bool {
	return len(data) >= webpHeaderSize && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

// EmbedMetadata returns encoded image data with the metadata added to it
// when part of it cannot be stored the rest is still written, and the error says what was left out
func EmbedMetadata(name string, data []byte, m Metadata) ([]byte, error) {
	switch name {
	case "jpeg", "jpg":
		return embedJPEGMetadata(data, m)
	case "png":
		return embedPNGMetadata(data, m)
	case "webp":
		return embedWebPMetadata(data, m)
	}
	return data, fmt.Errorf("%s cannot store metadata", name)
}

// every EXIF block starts with a TIFF header giving its byte order
func exifByteOrder(exif []byte) binary.ByteOrder {
	switch {
	case len(exif) < 8:
		return nil
	case string(exif[:4]) == "II*\x00":
		return binary.LittleEndian
	case string(exif[:4]) == "MM\x00*":
		return binary.BigEndian
	}
	return nil
}

func validEXIF(exif []byte) bool {
	return len(exif) <= maxMetadataSize && exifByteOrder(exif) != nil
}

// UprightEXIF returns a copy of an EXIF block that matches pixels which have been turned upright and resized
// the orientation tag is set to normal, as viewers would otherwise turn the already corrected image a second time
// the embedded thumbnail is unlinked, as it still shows the picture as it was before
func UprightEXIF(exif []byte) []byte {
	order := exifByteOrder(exif)
	if order == nil {
		return exif
	}
	out := bytes.Clone(exif)

	//the first directory holds the main image's tags as 12 byte entries, followed by a link to the thumbnail's directory
	dir := int64(order.Uint32(out[4:8]))
	if dir+2 > int64(len(out)) {
		return out
	}
	count := int64(order.Uint16(out[dir : dir+2]))
	end := dir + 2 + count*12
	if end+4 > int64(len(out)) {
		return out
	}

	const orientationTag, shortType = 0x0112, 3
	for entry := dir + 2; entry < end; entry += 12 {
		if order.Uint16(out[entry:]) == orientationTag && order.Uint16(out[entry+2:]) == shortType {
			order.PutUint16(out[entry+8:], 1)
		}
	}
	order.PutUint32(out[end:], 0)
	return out
}

// ---- JPEG ----
// a JPEG is a list of segments: FF, a marker byte, then a two byte length that counts itself and the payload

const (
	jpegAPP1 = 0xE1 //EXIF
	jpegAPP2 = 0xE2 //ICC profile, split over several segments when large
	jpegSOS  = 0xDA //start of the compressed image, no metadata follows

	jpegEXIFHeader = "Exif\x00\x00"
	jpegICCHeader  = "ICC_PROFILE\x00"
	//a segment holds 65535 bytes including its length field
	jpegMaxPayload = 65533
)

func readJPEGMetadata(data []byte) Metadata {
	var m Metadata
	iccParts := make(map[byte][]byte)
	iccTotal := byte(0)

	for pos := 2; pos+4 <= len(data) && data[pos] == 0xFF; {
		marker := data[pos+1]
		if marker == jpegSOS {
			break
		}
		length := int(binary.BigEndian.Uint16(data[pos+2:]))
		if length < 2 || pos+2+length > len(data) {
			break
		}
		payload := data[pos+4 : pos+2+length]

		switch {
		case marker == jpegAPP1 && m.EXIF == nil && bytes.HasPrefix(payload, []byte(jpegEXIFHeader)):
			m.EXIF = payload[len(jpegEXIFHeader):]
		case marker == jpegAPP2 && bytes.HasPrefix(payload, []byte(jpegICCHeader)) && len(payload) > len(jpegICCHeader)+2:
			//each part says which one it is and how many there are
			rest := payload[len(jpegICCHeader):]
			iccParts[rest[0]] = rest[2:]
			iccTotal = rest[1]
		}
		pos += 2 + length
	}

	//a profile with a part missing is useless
	for i := byte(1); i <= iccTotal && iccTotal > 0; i++ {
		part, ok := iccParts[i]
		if !ok {
			m.ICC = nil
			break
		}
		m.ICC = append(m.ICC, part...)
	}
	return m
}

func embedJPEGMetadata(data []byte, m Metadata) ([]byte, error) {
	if !isJPEG(data) {
		return data, errors.New("not a JPEG")
	}
	var err error
	var segments bytes.Buffer
	segment := func(marker byte, payload ...[]byte) {
		length := 2
		for _, p := range payload {
			length += len(p)
		}
		segments.Write([]byte{0xFF, marker, byte(length >> 8), byte(length)})
		for _, p := range payload {
			segments.Write(p)
		}
	}

	if len(m.EXIF) > 0 {
		if len(m.EXIF)+len(jpegEXIFHeader) > jpegMaxPayload {
			err = errors.New("EXIF data is too large for a JPEG and was left out")
		} else {
			segment(jpegAPP1, []byte(jpegEXIFHeader), m.EXIF)
		}
	}

	if len(m.ICC) > 0 {
		partSize := jpegMaxPayload - len(jpegICCHeader) - 2
		parts := (len(m.ICC) + partSize - 1) / partSize
		if parts > 255 {
			err = errors.Join(err, errors.New("colour profile is too large for a JPEG and was left out"))
		} else {
			for i := 0; i < parts; i++ {
				part := m.ICC[i*partSize : min(len(m.ICC), (i+1)*partSize)]
				segment(jpegAPP2, []byte(jpegICCHeader), []byte{byte(i + 1), byte(parts)}, part)
			}
		}
	}

	//straight after the two byte start of image marker, where readers expect them
	out := make([]byte, 0, len(data)+segments.Len())
	out = append(out, data[:2]...)
	out = append(out, segments.Bytes()...)
	out = append(out, data[2:]...)
	return out, err
}

// ---- PNG ----
// a PNG is a signature followed by chunks: a four byte length, a four letter type, the data and a checksum

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

func readPNGMetadata(data []byte) Metadata {
	var m Metadata
	for pos := len(pngSignature); pos+12 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[pos:]))
		kind := string(data[pos+4 : pos+8])
		if length < 0 || pos+12+length > len(data) {
			break
		}
		payload := data[pos+8 : pos+8+length]

		switch kind {
		case "eXIf":
			m.EXIF = payload
		case "iCCP":
			//a name, a zero byte, a compression method byte, then the profile compressed with zlib
			if i := bytes.IndexByte(payload, 0); i >= 0 && i+2 <= len(payload) {
				if r, err := zlib.NewReader(bytes.NewReader(payload[i+2:])); err == nil {
					if icc, err := io.ReadAll(io.LimitReader(r, maxMetadataSize+1)); err == nil {
						m.ICC = icc
					}
				}
			}
		}
		pos += 12 + length
	}
	return m
}

func embedPNGMetadata(data []byte, m Metadata) ([]byte, error) {
	//the header chunk comes first and is always 13 bytes long, the profile has to come before any pixel data
	const afterHeader = 8 + 12 + 13
	if !bytes.HasPrefix(data, pngSignature) || len(data) < afterHeader || string(data[12:16]) != "IHDR" {
		return data, errors.New("not a PNG")
	}

	var chunks bytes.Buffer
	chunk := func(kind string, payload []byte) {
		binary.Write(&chunks, binary.BigEndian, uint32(len(payload)))
		body := append([]byte(kind), payload...)
		chunks.Write(body)
		binary.Write(&chunks, binary.BigEndian, crc32.ChecksumIEEE(body))
	}

	if len(m.ICC) > 0 {
		var payload bytes.Buffer
		payload.WriteString("ICC Profile\x00\x00")
		z := zlib.NewWriter(&payload)
		z.Write(m.ICC)
		z.Close()
		chunk("iCCP", payload.Bytes())
	}
	if len(m.EXIF) > 0 {
		chunk("eXIf", m.EXIF)
	}

	out := make([]byte, 0, len(data)+chunks.Len())
	out = append(out, data[:afterHeader]...)
	out = append(out, chunks.Bytes()...)
	out = append(out, data[afterHeader:]...)
	return out, nil
}

// ---- WebP ----
// a WebP is a RIFF file: "RIFF", the size of what follows, "WEBP", then chunks of
// a four letter type, a four byte little-endian length and the data, padded to an even length

const (
	webpHeaderSize = 12
	//flags in the VP8X chunk announcing which optional chunks the file has
	webpFlagICC   = 0x20
	webpFlagAlpha = 0x10
	webpFlagEXIF  = 0x08
)

type webpChunk struct {
	kind    string
	payload []byte
}

func readWebPChunks(data []byte) []webpChunk {
	var chunks []webpChunk
	for pos := webpHeaderSize; pos+8 <= len(data); {
		length := int(binary.LittleEndian.Uint32(data[pos+4:]))
		if length < 0 || pos+8+length > len(data) {
			break
		}
		chunks = append(chunks, webpChunk{string(data[pos : pos+4]), data[pos+8 : pos+8+length]})
		pos += 8 + length + length%2
	}
	return chunks
}

func readWebPMetadata(data []byte) Metadata {
	var m Metadata
	for _, c := range readWebPChunks(data) {
		switch c.kind {
		case "ICCP":
			m.ICC = c.payload
		case "EXIF":
			//some writers wrongly include the header JPEG puts in front of the block
			m.EXIF = bytes.TrimPrefix(c.payload, []byte(jpegEXIFHeader))
		}
	}
	return m
}

func embedWebPMetadata(data []byte, m Metadata) ([]byte, error) {
	if !isWebP(data) {
		return data, errors.New("not a WebP")
	}
	chunks := readWebPChunks(data)
	if len(chunks) == 0 {
		return data, errors.New("not a WebP")
	}

	//metadata needs the extended layout, which starts with a VP8X chunk describing the file
	//the simple layout is just the image data, so the chunk is built from what the image data says
	if chunks[0].kind != "VP8X" {
		header, err := webpExtendedHeader(chunks[0])
		if err != nil {
			return data, err
		}
		chunks = append([]webpChunk{{"VP8X", header}}, chunks...)
	} else if len(chunks[0].payload) < 10 {
		return data, errors.New("damaged WebP header")
	}
	header := bytes.Clone(chunks[0].payload)

	//the profile goes straight after the header, EXIF after the image data
	var out []webpChunk
	out = append(out, webpChunk{"VP8X", header})
	if len(m.ICC) > 0 {
		header[0] |= webpFlagICC
		out = append(out, webpChunk{"ICCP", m.ICC})
	}
	for _, c := range chunks[1:] {
		if (c.kind == "ICCP" && len(m.ICC) > 0) || (c.kind == "EXIF" && len(m.EXIF) > 0) {
			continue
		}
		out = append(out, c)
	}
	if len(m.EXIF) > 0 {
		header[0] |= webpFlagEXIF
		out = append(out, webpChunk{"EXIF", m.EXIF})
	}

	var body bytes.Buffer
	body.WriteString("WEBP")
	for _, c := range out {
		body.WriteString(c.kind)
		binary.Write(&body, binary.LittleEndian, uint32(len(c.payload)))
		body.Write(c.payload)
		if len(c.payload)%2 == 1 {
			body.WriteByte(0)
		}
	}

	file := make([]byte, 0, 8+body.Len())
	file = append(file, "RIFF"...)
	file = binary.LittleEndian.AppendUint32(file, uint32(body.Len()))
	return append(file, body.Bytes()...), nil
}

// webpExtendedHeader builds the VP8X payload for a file that so far holds only image data:
// a flags byte, three reserved bytes, then the width and height, each minus one, in three bytes
func webpExtendedHeader(image webpChunk) ([]byte, error) {
	var width, height int
	var flags byte
	p := image.payload

	switch {
	case image.kind == "VP8 " && len(p) >= 10 && p[3] == 0x9D && p[4] == 0x01 && p[5] == 0x2A:
		//lossy: a three byte frame tag, a start code, then 14 bit sizes
		width = int(binary.LittleEndian.Uint16(p[6:]) & 0x3FFF)
		height = int(binary.LittleEndian.Uint16(p[8:]) & 0x3FFF)
	case image.kind == "VP8L" && len(p) >= 5 && p[0] == 0x2F:
		//lossless: a signature byte, then 14 bits each for width and height minus one, and an alpha bit
		bits := binary.LittleEndian.Uint32(p[1:])
		width = int(bits&0x3FFF) + 1
		height = int(bits>>14&0x3FFF) + 1
		if bits>>28&1 == 1 {
			flags |= webpFlagAlpha
		}
	default:
		return nil, errors.New("unrecognised WebP image data")
	}
	if width < 1 || height < 1 {
		return nil, errors.New("unrecognised WebP image data")
	}

	w, h := width-1, height-1
	return []byte{flags, 0, 0, 0, byte(w), byte(w >> 8), byte(w >> 16), byte(h), byte(h >> 8), byte(h >> 16)}, nil
}
