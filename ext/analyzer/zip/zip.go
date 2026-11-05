// Package zip is the analyzer "zip" of goda-hex.vim, as ddx-analyzer-zip:
// the headers of a zip archive (the local file headers, the data
// descriptors, the central directory and its end).
package zip

import (
	"github.com/Shougo/goda-hex.vim/hex"
)

// Name is the name of the analyzer.
const Name = "zip"

type analyzer struct{}

// New returns the analyzer.
func New() hex.Analyzer { return analyzer{} }

func (analyzer) Name() string { return Name }

func (analyzer) Detect(b *hex.Buffer, _ hex.Params) bool {
	return string(b.Bytes(b.Offset(), 2)) == "PK"
}

func (analyzer) Parse(b *hex.Buffer, _ hex.Params) ([]hex.AnalyzeResult, error) {
	var results []hex.AnalyzeResult
	addr := b.Offset()
	for {
		var h *hex.Header
		switch string(b.Bytes(addr, 4)) {
		case "PK\x03\x04":
			h = localHeader(b, addr)
		case "PK\x07\x08":
			h = hex.NewHeader(b, "ZIP_HEADER(PK78)", addr, true)
			h.Bytes("signature", 4)
			h.Line("uint32_t crc32;")
			h.Line("uint32_t compressed_size;")
			h.Line("uint32_t uncompressed_size;")
		case "PK\x01\x02":
			h = centralHeader(b, addr)
		case "PK\x05\x06":
			h = hex.NewHeader(b, "ZIP_END_HEADER", addr, true)
			h.Bytes("signature", 4)
			h.Line("uint16_t number_of_disks;")
			h.Line("uint16_t disk_number_start;")
			h.Line("uint16_t number_of_disk_entries;")
			h.Line("uint16_t number_of_entries;")
			h.Line("uint32_t central_dir_size;")
			h.Line("uint32_t central_dir_offset;")
			comment := h.Line("uint16_t file_comment_length;")
			h.String("comment", int(comment), hex.UTF8)
		default:
			if results == nil {
				results = []hex.AnalyzeResult{}
			}
			return results, nil
		}
		if h.Err != nil || h.Addr <= addr {
			// Out of the part of the file read (the option "length").
			return results, nil
		}
		results = append(results, h.Result)
		addr = h.Addr
	}
}

func localHeader(b *hex.Buffer, addr int64) *hex.Header {
	h := hex.NewHeader(b, "ZIP_HEADER", addr, true)
	h.Bytes("signature", 4)
	h.Line("uint16_t version;")
	flags := h.Line("uint16_t flags;")
	h.Line("uint16_t compression;")
	h.Line("uint16_t dos_time;")
	h.Line("uint16_t dos_date;")
	h.Line("uint32_t crc32;")
	compressed := h.Line("uint32_t compressed_size;")
	uncompressed := h.Line("uint32_t uncompressed_size;")
	nameLen := h.Line("uint16_t file_name_length;")
	extraLen := h.Line("uint16_t extra_field_length;")
	h.String("filename", int(nameLen), hex.UTF8)
	h.Mark("extra field", extraLen)
	size := compressed
	if size == 0 {
		size = uncompressed
	}
	if size == 0 && flags&0x8 != 0 && h.Err == nil {
		// The sizes are in the data descriptor after the data.
		if at := b.Search(h.Addr, hex.Pattern([]byte("PK\x07\x08")), true); at >= 0 {
			size = at - h.Addr
		}
	}
	h.Mark("data", size)
	return h
}

func centralHeader(b *hex.Buffer, addr int64) *hex.Header {
	h := hex.NewHeader(b, "ZIP_CENTRAL_HEADER", addr, true)
	h.Bytes("signature", 4)
	h.Line("uint16_t version_made;")
	h.Line("uint16_t version;")
	h.Line("uint16_t flags;")
	h.Line("uint16_t compression;")
	h.Line("uint16_t dos_time;")
	h.Line("uint16_t dos_date;")
	h.Line("uint32_t crc32;")
	h.Line("uint32_t compressed_size;")
	h.Line("uint32_t uncompressed_size;")
	nameLen := h.Line("uint16_t file_name_length;")
	extraLen := h.Line("uint16_t extra_field_length;")
	commentLen := h.Line("uint16_t file_comment_length;")
	h.Line("uint16_t disk_number_start;")
	h.Line("uint16_t internal_file_attributes;")
	h.Line("uint32_t external_file_attributes;")
	h.Line("uint32_t position;")
	h.String("filename", int(nameLen), hex.UTF8)
	h.Mark("extra field", extraLen)
	h.String("comment", int(commentLen), hex.UTF8)
	return h
}
