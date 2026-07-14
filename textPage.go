package izapplebasic

/*
The real COUT1 writes the characters to the text page memory, but it
is intercepted. To have valid content on the screen snapshots, the
intercepted output is mirrored to the text page like the real
routine would do, using the cursor positions CH and CV, respecting
INVFLG and scrolling the page when the bottom is reached.

The output does not wrap at 40 columns as a real Apple II would, the
chars beyond the right border are not mirrored.
*/

const (
	textColumns = 40
	textRows    = 24

	zpWNDTOP = uint16(0x22) // First row of the text window
	zpWNDBTM = uint16(0x23) // One past the last row of the text window
	zpINVFLG = uint16(0x32) // Mask for inverse or flashing text
)

/*
textWindow returns the vertical text window limits. The text and the
lores graphics share the same memory: in mixed graphics modes the
text is restricted to the bottom rows, scrolling there must not
touch the graphics rows. Applesoft GR sets the window to rows 20-23
and TEXT restores the full screen.
*/
func (env *Environment) textWindow() (uint8, uint8) {
	top := env.mem.Peek(zpWNDTOP)
	bottom := env.mem.Peek(zpWNDBTM)
	if bottom > textRows || top >= bottom {
		return 0, textRows
	}
	return top, bottom
}

// textPageRowAddress returns the address of the first char of a
// row, with the classic Apple II interleaved layout.
func textPageRowAddress(row uint8) uint16 {
	return textPage1Address + uint16(row&7)*0x80 + uint16(row>>3)*0x28
}

func (env *Environment) textPagePutChar(ch uint8) {
	col := env.col
	if col >= textColumns {
		return
	}
	row := env.mem.Peek(zpCV)
	if row >= textRows {
		row = textRows - 1
	}
	// The high bit set is a normal char, INVFLG clears bits to make
	// it inverse (0x3f) or flashing (0x7f)
	value := (ch | 0x80) & env.mem.Peek(zpINVFLG)
	env.mem.pokeHost(textPageRowAddress(row)+uint16(col), value)
}

func (env *Environment) textPageNewLine() {
	_, bottom := env.textWindow()
	row := env.mem.Peek(zpCV)
	if row < bottom-1 {
		env.mem.Poke(zpCV, row+1)
		return
	}
	env.mem.Poke(zpCV, bottom-1)
	env.textPageScroll()
}

func (env *Environment) textPageScroll() {
	top, bottom := env.textWindow()
	for row := top; row < bottom-1; row++ {
		src := textPageRowAddress(row + 1)
		dst := textPageRowAddress(row)
		for col := uint16(0); col < textColumns; col++ {
			env.mem.pokeHost(dst+col, env.mem.Peek(src+col))
		}
	}
	env.textPageClearRow(bottom - 1)
}

func (env *Environment) textPageClearRow(row uint8) {
	address := textPageRowAddress(row)
	for col := uint16(0); col < textColumns; col++ {
		env.mem.pokeHost(address+col, 0xa0) // space
	}
}

func (env *Environment) textPageClear() {
	top, bottom := env.textWindow()
	for row := top; row < bottom; row++ {
		env.textPageClearRow(row)
	}
	env.mem.Poke(zpCV, top)
}

// currentLine returns the text of the cursor row up to the cursor
// column: when input is requested, this is the pending prompt.
func (env *Environment) currentLine() string {
	row := env.mem.Peek(zpCV)
	if row >= textRows {
		row = textRows - 1
	}
	col := int(env.col)
	if col > textColumns {
		col = textColumns
	}
	address := textPageRowAddress(row)
	line := make([]uint8, col)
	for i := 0; i < col; i++ {
		line[i] = env.mem.Peek(address+uint16(i)) & 0x7f
	}
	return string(line)
}
