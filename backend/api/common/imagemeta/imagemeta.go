// Package imagemeta removes metadata from uploaded images without
// re-encoding their pixels.
package imagemeta

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// ErrInvalidImage reports an upload whose container structure cannot be
// parsed, so its metadata cannot be removed safely.
var ErrInvalidImage = errors.New("invalid image file")

// Strip removes metadata (Exif incl. GPS, XMP, IPTC, comments,
// text chunks, appended secondary images) from a JPEG, PNG or WebP file
// without re-encoding the pixels. Only the Exif orientation survives, as a
// minimal Exif block holding that single tag, so viewers keep showing the
// photo upright. Colour data (ICC profile, Adobe transform, sRGB/gamma) stays.
func Strip(data []byte, contentType string) ([]byte, error) {
	switch contentType {
	case "image/jpeg", "image/jpg":
		return stripJPEGMetadata(data)
	case "image/png":
		return stripPNGMetadata(data)
	case "image/webp":
		return stripWebPMetadata(data)
	default:
		return nil, ErrInvalidImage
	}
}

// --- Exif orientation -------------------------------------------------------

const exifOrientationTag = 0x0112

// exifOrientation reads the IFD0 orientation tag of a TIFF-structured Exif
// payload. It returns 0 when the tag is absent or unreadable.
func exifOrientation(tiff []byte) uint16 {
	order, ok := tiffByteOrder(tiff)
	if !ok {
		return 0
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[ifd : ifd+2]))
	for i := range count {
		entry := ifd + 2 + i*12
		if entry+12 > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[entry:entry+2]) == exifOrientationTag {
			return orientationEntryValue(order, tiff[entry:entry+12])
		}
	}
	return 0
}

// tiffByteOrder validates the TIFF header and returns its byte order.
func tiffByteOrder(tiff []byte) (binary.ByteOrder, bool) {
	if len(tiff) < 8 {
		return nil, false
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, false
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return nil, false
	}
	return order, true
}

// orientationEntryValue reads a 12-byte orientation IFD entry. It returns 0
// unless the entry is a single SHORT holding a valid orientation (1-8).
func orientationEntryValue(order binary.ByteOrder, entry []byte) uint16 {
	if order.Uint16(entry[2:4]) != 3 || order.Uint32(entry[4:8]) != 1 {
		return 0
	}
	value := order.Uint16(entry[8:10])
	if value < 1 || value > 8 {
		return 0
	}
	return value
}

// orientationOnlyTIFF builds a big-endian TIFF block whose IFD0 holds only
// the orientation tag.
func orientationOnlyTIFF(orientation uint16) []byte {
	out := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1}
	entry := make([]byte, 12)
	binary.BigEndian.PutUint16(entry[0:2], exifOrientationTag)
	binary.BigEndian.PutUint16(entry[2:4], 3) // SHORT
	binary.BigEndian.PutUint32(entry[4:8], 1)
	binary.BigEndian.PutUint16(entry[8:10], orientation)
	out = append(out, entry...)
	return append(out, 0, 0, 0, 0) // no next IFD
}

// --- JPEG -------------------------------------------------------------------

var exifHeader = []byte("Exif\x00\x00")

func stripJPEGMetadata(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, ErrInvalidImage
	}
	s := jpegStripper{data: data, pos: 2, out: make([]byte, 0, len(data))}
	s.out = append(s.out, 0xFF, 0xD8)
	for {
		done, err := s.step()
		if err != nil {
			return nil, err
		}
		if done {
			return s.out, nil
		}
	}
}

// jpegStripper copies a JPEG marker by marker, dropping metadata segments.
type jpegStripper struct {
	data               []byte
	pos                int
	out                []byte
	orientationWritten bool
}

// step processes the next marker and reports whether EOI was reached.
func (s *jpegStripper) step() (bool, error) {
	marker, next, err := nextJPEGMarker(s.data, s.pos)
	if err != nil {
		return false, err
	}
	s.pos = next
	if marker == 0xD9 { // EOI: anything after it (e.g. MPF images) is dropped
		s.out = append(s.out, 0xFF, 0xD9)
		return true, nil
	}
	if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
		s.out = append(s.out, 0xFF, marker)
		return false, nil
	}
	payload, segment, err := s.readSegment()
	if err != nil {
		return false, err
	}
	return false, s.handleSegment(marker, payload, segment)
}

// readSegment reads the length-prefixed segment at pos, whose marker was just
// consumed, and returns its payload and the full segment including the marker.
func (s *jpegStripper) readSegment() (payload, segment []byte, err error) {
	pos := s.pos
	if pos+2 > len(s.data) {
		return nil, nil, ErrInvalidImage
	}
	length := int(binary.BigEndian.Uint16(s.data[pos : pos+2]))
	if length < 2 || pos+length > len(s.data) {
		return nil, nil, ErrInvalidImage
	}
	s.pos = pos + length
	return s.data[pos+2 : pos+length], s.data[pos-2 : pos+length], nil
}

func (s *jpegStripper) handleSegment(marker byte, payload, segment []byte) error {
	switch {
	case marker == 0xE1 && bytes.HasPrefix(payload, exifHeader):
		s.writeOrientation(payload[len(exifHeader):])
	case marker >= 0xE0 && marker <= 0xEF, marker == 0xFE:
		if keepJPEGAppSegment(marker, payload) {
			s.out = append(s.out, segment...)
		}
	case marker == 0xDA: // SOS: copy the header and the entropy-coded scan
		s.out = append(s.out, segment...)
		end, err := jpegScanEnd(s.data, s.pos)
		if err != nil {
			return err
		}
		s.out = append(s.out, s.data[s.pos:end]...)
		s.pos = end
	default:
		s.out = append(s.out, segment...)
	}
	return nil
}

// writeOrientation emits a minimal Exif segment for the first non-default
// orientation found.
func (s *jpegStripper) writeOrientation(tiff []byte) {
	if s.orientationWritten {
		return
	}
	if orientation := exifOrientation(tiff); orientation > 1 {
		s.out = appendJPEGSegment(s.out, 0xE1, append(append([]byte{}, exifHeader...), orientationOnlyTIFF(orientation)...))
		s.orientationWritten = true
	}
}

// keepJPEGAppSegment keeps the APP segments a decoder needs to show the
// pixels correctly; every other APPn and COM segment is metadata.
func keepJPEGAppSegment(marker byte, payload []byte) bool {
	switch marker {
	case 0xE0:
		return bytes.HasPrefix(payload, []byte("JFIF\x00"))
	case 0xE2:
		return bytes.HasPrefix(payload, []byte("ICC_PROFILE\x00"))
	case 0xEE:
		return bytes.HasPrefix(payload, []byte("Adobe"))
	default:
		return false
	}
}

// nextJPEGMarker expects a marker at pos (fill bytes allowed) and returns the
// marker code and the offset after it.
func nextJPEGMarker(data []byte, pos int) (byte, int, error) {
	if pos >= len(data) || data[pos] != 0xFF {
		return 0, 0, ErrInvalidImage
	}
	for pos < len(data) && data[pos] == 0xFF {
		pos++
	}
	if pos >= len(data) {
		return 0, 0, ErrInvalidImage
	}
	return data[pos], pos + 1, nil
}

// jpegScanEnd returns the offset of the first marker after entropy-coded
// data, skipping byte stuffing (FF 00) and restart markers.
func jpegScanEnd(data []byte, pos int) (int, error) {
	for i := pos; i+1 < len(data); i++ {
		if data[i] != 0xFF {
			continue
		}
		next := data[i+1]
		if next == 0x00 || next == 0xFF || (next >= 0xD0 && next <= 0xD7) {
			continue
		}
		return i, nil
	}
	return 0, ErrInvalidImage
}

func appendJPEGSegment(out []byte, marker byte, payload []byte) []byte {
	out = append(out, 0xFF, marker)
	out = binary.BigEndian.AppendUint16(out, uint16(len(payload)+2))
	return append(out, payload...)
}

// --- PNG --------------------------------------------------------------------

var pngSignatureBytes = []byte("\x89PNG\r\n\x1a\n")

// pngKeptAncillary are ancillary chunks that affect rendering. Other
// ancillary chunks (tEXt, zTXt, iTXt, tIME, eXIf, private chunks) are dropped.
var pngKeptAncillary = map[string]bool{
	"tRNS": true, "cHRM": true, "gAMA": true, "iCCP": true, "sBIT": true,
	"sRGB": true, "cICP": true, "mDCV": true, "cLLI": true, "bKGD": true,
	"hIST": true, "pHYs": true, "sPLT": true, "acTL": true, "fcTL": true,
	"fdAT": true,
}

func stripPNGMetadata(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, pngSignatureBytes) {
		return nil, ErrInvalidImage
	}
	out := make([]byte, 0, len(data))
	out = append(out, pngSignatureBytes...)
	pos := len(pngSignatureBytes)
	orientationWritten := false
	for pos+12 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if length < 0 || pos+12+length > len(data) {
			return nil, ErrInvalidImage
		}
		chunkType := string(data[pos+4 : pos+8])
		chunk := data[pos : pos+12+length]
		payload := data[pos+8 : pos+8+length]
		pos += 12 + length

		switch {
		case chunkType == "eXIf":
			if orientation := exifOrientation(payload); orientation > 1 && !orientationWritten {
				out = appendPNGChunk(out, "eXIf", orientationOnlyTIFF(orientation))
				orientationWritten = true
			}
		case chunkType[0] >= 'A' && chunkType[0] <= 'Z', pngKeptAncillary[chunkType]:
			out = append(out, chunk...)
		}
		if chunkType == "IEND" {
			return out, nil
		}
	}
	return nil, ErrInvalidImage
}

func appendPNGChunk(out []byte, chunkType string, payload []byte) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(payload)))
	start := len(out)
	out = append(out, chunkType...)
	out = append(out, payload...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[start:]))
}

// --- WebP -------------------------------------------------------------------

// webpKeptChunks are the image and rendering chunks; EXIF, XMP and unknown
// chunks are dropped.
var webpKeptChunks = map[string]bool{
	"VP8 ": true, "VP8L": true, "VP8X": true, "ALPH": true,
	"ANIM": true, "ANMF": true, "ICCP": true,
}

const (
	webpFlagEXIF = 0x08
	webpFlagXMP  = 0x04
)

func stripWebPMetadata(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, ErrInvalidImage
	}
	riffEnd := 8 + int(binary.LittleEndian.Uint32(data[4:8]))
	if riffEnd > len(data) {
		return nil, ErrInvalidImage
	}
	s := webpStripper{out: make([]byte, 12, len(data)), vp8xFlags: -1}
	copy(s.out, data[:12])
	pos := 12
	for pos < riffEnd {
		c, err := readWebPChunk(data, pos, riffEnd)
		if err != nil {
			return nil, err
		}
		pos = c.next
		if err := s.handleChunk(c); err != nil {
			return nil, err
		}
	}
	return s.finish(), nil
}

// riffChunk is one RIFF chunk; raw includes header and padding.
type riffChunk struct {
	fourCC  string
	payload []byte
	raw     []byte
	next    int
}

func readWebPChunk(data []byte, pos, riffEnd int) (riffChunk, error) {
	if pos+8 > riffEnd {
		return riffChunk{}, ErrInvalidImage
	}
	size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
	padded := size + size&1
	if size < 0 || pos+8+size > riffEnd {
		return riffChunk{}, ErrInvalidImage
	}
	end := min(pos+8+padded, riffEnd)
	return riffChunk{
		fourCC:  string(data[pos : pos+4]),
		payload: data[pos+8 : pos+8+size],
		raw:     data[pos:end],
		next:    end,
	}, nil
}

// webpStripper collects the kept chunks and the orientation to re-emit.
type webpStripper struct {
	out              []byte
	vp8xFlags        int // offset of the VP8X flags byte in out, or -1
	orientationChunk []byte
}

func (s *webpStripper) handleChunk(c riffChunk) error {
	switch {
	case c.fourCC == "EXIF":
		tiff := bytes.TrimPrefix(c.payload, exifHeader)
		if orientation := exifOrientation(tiff); orientation > 1 && s.orientationChunk == nil {
			s.orientationChunk = orientationOnlyTIFF(orientation)
		}
	case webpKeptChunks[c.fourCC]:
		if c.fourCC == "VP8X" {
			if len(c.payload) < 1 {
				return ErrInvalidImage
			}
			s.vp8xFlags = len(s.out) + 8
		}
		s.out = append(s.out, c.raw...)
	}
	return nil
}

// finish fixes the VP8X metadata flags, appends the orientation chunk and
// rewrites the RIFF size.
func (s *webpStripper) finish() []byte {
	out := s.out
	if s.vp8xFlags >= 0 {
		out[s.vp8xFlags] &^= webpFlagEXIF | webpFlagXMP
		if s.orientationChunk != nil {
			out[s.vp8xFlags] |= webpFlagEXIF
			// EXIF belongs after the image data, at the end of the file.
			out = append(out, "EXIF"...)
			out = binary.LittleEndian.AppendUint32(out, uint32(len(s.orientationChunk)))
			out = append(out, s.orientationChunk...)
		}
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out
}
