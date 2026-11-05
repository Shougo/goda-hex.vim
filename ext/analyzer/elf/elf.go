// Package elf is the analyzer "elf" of goda-hex.vim, as ddx-analyzer-elf:
// the ELF header and the program headers of an ELF binary (32 or 64 bits,
// little or big endian).
package elf

import (
	"fmt"

	"github.com/Shougo/goda-hex.vim/hex"
)

// Name is the name of the analyzer.
const Name = "elf"

type analyzer struct{}

// New returns the analyzer.
func New() hex.Analyzer { return analyzer{} }

func (analyzer) Name() string { return Name }

func (analyzer) Detect(b *hex.Buffer, _ hex.Params) bool {
	return string(b.Bytes(b.Offset(), 4)) == "\x7fELF"
}

func (analyzer) Parse(b *hex.Buffer, _ hex.Params) ([]hex.AnalyzeResult, error) {
	start := b.Offset()
	ident := b.Bytes(start, 16)
	if len(ident) < 16 {
		return nil, fmt.Errorf("no ELF identification")
	}
	is64 := ident[4] == 2
	little := ident[5] != 2
	word := "uint32_t"
	if is64 {
		word = "uint64_t"
	}
	h := hex.NewHeader(b, "ELF_HEADER", start, little)
	h.Bytes("e_ident", 16)
	h.Line("uint16_t e_type;")
	h.Line("uint16_t e_machine;")
	h.Line("uint32_t e_version;")
	h.Line(word + " e_entry;")
	phoff := h.Line(word + " e_phoff;")
	h.Line(word + " e_shoff;")
	h.Line("uint32_t e_flags;")
	h.Line("uint16_t e_ehsize;")
	h.Line("uint16_t e_phentsize;")
	phnum := h.Line("uint16_t e_phnum;")
	h.Line("uint16_t e_shentsize;")
	h.Line("uint16_t e_shnum;")
	h.Line("uint16_t e_shstrndx;")
	if h.Err != nil {
		return nil, h.Err
	}
	results := []hex.AnalyzeResult{h.Result}
	addr := start + phoff
	for i := range phnum {
		h := hex.NewHeader(b, fmt.Sprintf("PROGRAM_HEADER[%d]", i), addr, little)
		h.Line("uint32_t p_type;")
		if is64 {
			h.Line("uint32_t p_flags;")
		}
		h.Line(word + " p_offset;")
		h.Line(word + " p_vaddr;")
		h.Line(word + " p_paddr;")
		h.Line(word + " p_filesz;")
		h.Line(word + " p_memsz;")
		if !is64 {
			h.Line("uint32_t p_flags;")
		}
		h.Line(word + " p_align;")
		if h.Err != nil {
			// Out of the part of the file read (the option "length").
			break
		}
		results = append(results, h.Result)
		addr = h.Addr
	}
	return results, nil
}
