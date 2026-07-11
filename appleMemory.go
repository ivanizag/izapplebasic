package main

import (
	"fmt"
)

/*
Memory map of an Apple II+ without any card in the slots:

	0x0000-0xbfff: 48 KB of RAM
	0xc000-0xc0ff: softswitches, reads return zero, writes are ignored
	0xc100-0xcfff: empty slot ROM space, reads return zero
	0xd000-0xffff: 12 KB ROM (Applesoft BASIC and the monitor)

Reading zeros in the slot ROM area makes the autostart monitor slot
scan fail to find a bootable card, falling back to BASIC.
*/
const (
	ioAreaStart   = uint16(0xc000)
	slotAreaStart = uint16(0xc100)
	romAreaStart  = uint16(0xd000)
	romSize       = 0x3000
)

type appleMemory struct {
	data    [65536]uint8
	traceIO bool
}

func newAppleMemory(rom []uint8) (*appleMemory, error) {
	var m appleMemory
	if len(rom) != romSize {
		return nil, fmt.Errorf("the ROM must be %v bytes long, it is %v bytes", romSize, len(rom))
	}
	copy(m.data[romAreaStart:], rom)
	return &m, nil
}

func (m *appleMemory) Peek(address uint16) uint8 {
	if address >= ioAreaStart && address < romAreaStart {
		if m.traceIO && address < slotAreaStart {
			fmt.Printf("[[[IO read $%04X]]]\n", address)
		}
		// Softswitches and empty slots. Bit 7 clear on 0xc000
		// reads as "no key pressed".
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
		// Softswitches are ignored and the ROM is not writable
		return
	}
	m.data[address] = value
}

// pokeROM patches the ROM, used to place RTS opcodes on the
// intercepted monitor entry points.
func (m *appleMemory) pokeROM(address uint16, value uint8) {
	m.data[address] = value
}
