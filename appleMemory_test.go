package izapplebasic

import "testing"

func TestMemoryROMSizeValidation(t *testing.T) {
	_, err := newAppleMemory(make([]uint8, 100), embeddedCharGen)
	if err == nil {
		t.Error("a ROM with a bad size must be rejected")
	}
	_, err = newAppleMemory(applesoftROM, embeddedCharGen)
	if err != nil {
		t.Errorf("the embedded ROM must be accepted: %v", err)
	}
}

func TestMemoryRAM(t *testing.T) {
	m, _ := newAppleMemory(applesoftROM, embeddedCharGen)
	m.Poke(0x1234, 0xda)
	if m.Peek(0x1234) != 0xda {
		t.Error("RAM must be writable")
	}
}

func TestMemoryROMIsNotWritable(t *testing.T) {
	m, _ := newAppleMemory(applesoftROM, embeddedCharGen)
	value := m.Peek(0xe000)
	m.Poke(0xe000, value+1)
	if m.Peek(0xe000) != value {
		t.Error("the ROM must not be writable")
	}
}

func TestMemoryROMContent(t *testing.T) {
	m, _ := newAppleMemory(applesoftROM, embeddedCharGen)
	// The reset vector points to the autostart monitor RESET
	if m.Peek(0xfffc) != 0x62 || m.Peek(0xfffd) != 0xfa {
		t.Error("the reset vector must be 0xfa62")
	}
}

func TestMemoryIOArea(t *testing.T) {
	m, _ := newAppleMemory(applesoftROM, embeddedCharGen)
	m.Poke(0xc000, 0xff)
	if m.Peek(0xc000) != 0 {
		t.Error("the softswitches must read zero")
	}
	if m.Peek(0xc600) != 0 {
		t.Error("the empty slots must read zero")
	}
}

func TestMemoryBreakPending(t *testing.T) {
	m, _ := newAppleMemory(applesoftROM, embeddedCharGen)
	if m.Peek(ioKeyboard) != 0 {
		t.Error("no key must be pressed initially")
	}
	m.breakPending.Store(true)
	if m.Peek(ioKeyboard) != 0x83 {
		t.Error("a pending break must read as control-C")
	}
	m.Peek(ioStrobe)
	if m.Peek(ioKeyboard) != 0 {
		t.Error("the strobe must clear the pending break")
	}
}

func TestMemoryPokeROM(t *testing.T) {
	m, _ := newAppleMemory(applesoftROM, embeddedCharGen)
	m.pokeROM(addrCOUT1, 0x60)
	if m.Peek(addrCOUT1) != 0x60 {
		t.Error("pokeROM must patch the ROM")
	}
}
