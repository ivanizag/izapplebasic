package izapplebasic

import (
	"fmt"
	"sync/atomic"
)

/*
Memory map of an Apple II+ without any card in the slots:

	0x0000-0xbfff: 48 KB of RAM
	0xc000-0xc0ff: softswitches, reads return zero, writes are ignored
	0xc100-0xcfff: empty slot ROM space, reads return zero
	0xd000-0xffff: 12 KB ROM (Applesoft BASIC and the monitor)

Reading zeros in the slot ROM area makes the autostart monitor slot
scan fail to find a bootable card, falling back to BASIC.

The Integer BASIC ROM is 8 KB and covers only 0xe000-0xffff, the
0xd000-0xdfff socket of the Programmer's Aid is left reading zeros.
*/
const (
	ioAreaStart   = uint16(0xc000)
	slotAreaStart = uint16(0xc100)
	romAreaStart  = uint16(0xd000)
	romSize       = 0x3000
	romAreaStart8 = uint16(0xe000)
	romSize8      = 0x2000
	charGenSize   = 0x800

	ioKeyboard = uint16(0xc000)
	ioStrobe   = uint16(0xc010)

	ioGraphics = uint16(0xc050)
	ioText     = uint16(0xc051)
	ioFullScrn = uint16(0xc052)
	ioMixed    = uint16(0xc053)
	ioPage1    = uint16(0xc054)
	ioPage2    = uint16(0xc055)
	ioLoRes    = uint16(0xc056)
	ioHiRes    = uint16(0xc057)
)

type appleMemory struct {
	data    [65536]uint8
	charGen []uint8
	traceIO bool

	// Video mode softswitches state, tracked to know what to render
	// on the snapshots
	textMode  bool
	mixedMode bool
	page2     bool
	hiResMode bool

	// graphicsDirty is set when the emulated code writes to the
	// graphics memory: the hires pages, or the lores page while in
	// a graphics mode. The frontends use it to know when to show a
	// snapshot.
	graphicsDirty bool

	// breakPending presents a control-C keypress on the keyboard
	// softswitch. Applesoft polls it between statements to break a
	// running program. Set from the control-C signal handler
	// goroutine.
	breakPending atomic.Bool
}

func newAppleMemory(rom []uint8, charGen []uint8) (*appleMemory, error) {
	var m appleMemory
	if len(charGen) != charGenSize {
		return nil, fmt.Errorf("the character generator ROM must be %v bytes long, it is %v bytes", charGenSize, len(charGen))
	}
	if err := m.loadROM(rom); err != nil {
		return nil, err
	}
	m.charGen = charGen
	m.textMode = true
	return &m, nil
}

// loadROM places a 12 KB ROM on 0xd000 or an 8 KB one on 0xe000,
// clearing the whole ROM area first so no leftovers of a previous
// ROM are left when swapping to a smaller one.
func (m *appleMemory) loadROM(rom []uint8) error {
	if len(rom) != romSize && len(rom) != romSize8 {
		return fmt.Errorf("the ROM must be %v or %v bytes long, it is %v bytes",
			romSize, romSize8, len(rom))
	}
	for i := range m.data[romAreaStart:] {
		m.data[int(romAreaStart)+i] = 0
	}
	if len(rom) == romSize {
		copy(m.data[romAreaStart:], rom)
	} else {
		copy(m.data[romAreaStart8:], rom)
	}
	return nil
}

// ioSwitch processes the side effects of an access, read or write,
// to the softswitches.
func (m *appleMemory) ioSwitch(address uint16) {
	switch address {
	case ioStrobe:
		m.breakPending.Store(false)
	case ioGraphics:
		m.textMode = false
	case ioText:
		m.textMode = true
	case ioFullScrn:
		m.mixedMode = false
	case ioMixed:
		m.mixedMode = true
	case ioPage1:
		m.page2 = false
	case ioPage2:
		m.page2 = true
	case ioLoRes:
		m.hiResMode = false
	case ioHiRes:
		m.hiResMode = true
	}
}

func (m *appleMemory) Peek(address uint16) uint8 {
	if address >= ioAreaStart && address < romAreaStart {
		if m.traceIO && address < slotAreaStart {
			fmt.Printf("[[[IO read $%04X]]]\n", address)
		}
		if address == ioKeyboard && m.breakPending.Load() {
			// A control-C keypress with the high bit set
			return 0x83
		}
		m.ioSwitch(address)
		// Elsewhere, softswitches and empty slots read zero. Bit 7
		// clear on 0xc000 reads as "no key pressed".
		return 0
	}
	return m.data[address]
}

func (m *appleMemory) PeekCode(address uint16) uint8 {
	return m.Peek(address)
}

func (m *appleMemory) Poke(address uint16, value uint8) {
	if address >= ioAreaStart {
		if m.traceIO && address < slotAreaStart {
			fmt.Printf("[[[IO write $%04X <= $%02X]]]\n", address, value)
		}
		m.ioSwitch(address)
		// The ROM is not writable
		return
	}
	if address >= hiResPage1Address && address < hiResPage2Address+hiResPageSize {
		m.graphicsDirty = true
	} else if !m.textMode && address >= textPage1Address && address < textPage1Address+textPageSize {
		m.graphicsDirty = true
	}
	m.data[address] = value
}

// pokeHost writes to the memory without the side effects of Poke.
// Used by the host text page mirroring, that must not mark the
// graphics as dirty.
func (m *appleMemory) pokeHost(address uint16, value uint8) {
	m.data[address] = value
}

// pokeROM patches the ROM, used to place RTS opcodes on the
// intercepted monitor entry points.
func (m *appleMemory) pokeROM(address uint16, value uint8) {
	m.data[address] = value
}
