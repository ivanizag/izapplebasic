package main

import (
	"image"
	"image/color"

	"github.com/ivanizag/izapple2/screen"
)

/*
appleMemory implements the screen.VideoSource interface of the
izapple2 screen module to render snapshots of the video memory.

The machine is an Apple II+: 40 columns, uppercase, no aux memory
and no super hires.
*/

const (
	textPage1Address  = uint16(0x0400)
	textPage2Address  = uint16(0x0800)
	textPageSize      = uint16(0x0400)
	hiResPage1Address = uint16(0x2000)
	hiResPage2Address = uint16(0x4000)
	hiResPageSize     = uint16(0x2000)
)

func (m *appleMemory) GetCurrentVideoMode() uint32 {
	var mode uint32
	if m.textMode {
		mode = screen.VideoText40
	} else {
		if m.hiResMode {
			mode = screen.VideoHGR
		} else {
			mode = screen.VideoGR
		}
		if m.mixedMode {
			mode |= screen.VideoMixText40
		}
	}
	if m.page2 {
		mode |= screen.VideoSecondPage
	}
	return mode
}

func (m *appleMemory) GetTextMemory(secondPage bool, ext bool) []uint8 {
	if ext {
		// No aux memory on the Apple II+
		return make([]uint8, textPageSize)
	}
	address := textPage1Address
	if secondPage {
		address = textPage2Address
	}
	return m.data[address : address+textPageSize]
}

func (m *appleMemory) GetVideoMemory(secondPage bool, ext bool) []uint8 {
	if ext {
		// No aux memory on the Apple II+
		return make([]uint8, hiResPageSize)
	}
	address := hiResPage1Address
	if secondPage {
		address = hiResPage2Address
	}
	return m.data[address : address+hiResPageSize]
}

func (m *appleMemory) GetCharacterPixel(char uint8, rowInChar int, colInChar int, isAltText bool, isFlashedFrame bool) bool {
	// Apple II+ character generator, as done in izapple2
	rowPos := (int(char)*8 + rowInChar) % len(m.charGen)
	bits := m.charGen[rowPos]
	pixel := (bits>>uint(6-colInChar))&1 == 1

	topBits := char >> 6
	isInverse := topBits == 0
	isFlash := topBits == 1
	return pixel != (isInverse || (isFlash && isFlashedFrame))
}

func (m *appleMemory) GetSuperVideoMemory() []uint8 {
	// No super hires on the Apple II+
	return nil
}

func (m *appleMemory) GetCardImage(light color.Color) *image.RGBA {
	// No video cards
	return nil
}

func (m *appleMemory) SupportsLowercase() bool {
	return false
}
