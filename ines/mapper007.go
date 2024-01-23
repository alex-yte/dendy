package ines

import (
	"errors"
	"log"

	"github.com/maxpoletaev/dendy/internal/binario"
)

// Mapper007 implements iNES mapper #7 aka AxROM.
// https://www.nesdev.org/wiki/AxROM
type Mapper007 struct {
	rom     *ROM
	prgBank uint8
	chrBank uint8
}

func NewMapper7(rom *ROM) *Mapper007 {
	return &Mapper007{
		rom: rom,
	}
}

func (m *Mapper007) ScanlineTick() {}

func (m *Mapper007) PendingIRQ() bool {
	return false
}

func (m *Mapper007) MirrorMode() MirrorMode {
	switch m.chrBank {
	case 0:
		return MirrorSingle0
	case 1:
		return MirrorSingle1
	default:
		panic("mapper7: invalid mirror mode")
	}
}

func (m *Mapper007) Reset() {
	m.prgBank = 0
	m.chrBank = 0
}

func (m *Mapper007) WritePRG(addr uint16, data byte) {
	if addr >= 0x8000 && addr <= 0xFFFF {
		m.chrBank = (data & 0x10) >> 4
		m.prgBank = data & 0x07
	} else {
		log.Printf("[WARN] mapper7: unhandled prg write at %04X", addr)
	}
}

func (m *Mapper007) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000 && addr <= 0xFFFF:
		offset := int(addr-0x8000) % 0x8000
		return m.rom.PRG[int(m.prgBank)*0x8000+offset]
	default:
		log.Printf("[WARN] mapper7: unhandled prg read at %04X", addr)
		return 0
	}
}

func (m *Mapper007) WriteCHR(addr uint16, data byte) {
	if !m.rom.chrRAM {
		log.Printf("[WARN] mapper4: write to read-only chr at %04X", addr)
		return
	}

	switch {
	case addr >= 0x0000 && addr <= 0x1FFF:
		m.rom.CHR[int(addr)%len(m.rom.CHR)] = data
	default:
		log.Printf("[WARN] mapper7: invalid chr write at %04X", addr)
	}
}

func (m *Mapper007) ReadCHR(addr uint16) byte {
	switch {
	case addr >= 0x0000 && addr <= 0x1FFF:
		return m.rom.CHR[int(addr)%len(m.rom.CHR)]
	default:
		log.Printf("[WARN] mapper7: invalid chr read at %04X", addr)
		return 0
	}
}

func (m *Mapper007) SaveState(w *binario.Writer) error {
	return errors.Join(
		w.WriteUint8(m.prgBank),
		w.WriteUint8(m.chrBank),
	)
}

func (m *Mapper007) LoadState(r *binario.Reader) error {
	return errors.Join(
		r.ReadUint8To(&m.prgBank),
		r.ReadUint8To(&m.chrBank),
	)
}
