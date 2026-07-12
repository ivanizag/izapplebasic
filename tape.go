package izapplebasic

import "fmt"

/*
Cassette tape emulation. The monitor WRITE (0xfecd) and READ
(0xfefd) routines are intercepted: each WRITE call produces one
checksummed record on a real tape, here one block handed to the
frontend. Applesoft SAVE writes two blocks, a length header and the
program, and LOAD reads them back. STORE, RECALL and SHLOAD also go
through these two entry points.

A record on a real tape has no length: READ reads as many bytes as
the memory range of the caller asks for and then fails the checksum
if that did not match the recording. On a size mismatch the read
here is truncated and the monitor "ERR" is shown.
*/

// tapeRange returns the memory range A1..A2 used by the monitor
// WRITE and READ calls.
func tapeRange(env *Environment) (uint16, int) {
	a1 := uint16(env.mem.Peek(zpA1L)) + uint16(env.mem.Peek(zpA1L+1))<<8
	a2 := uint16(env.mem.Peek(zpA2L)) + uint16(env.mem.Peek(zpA2L+1))<<8
	size := int(a2) - int(a1) + 1
	if size < 1 {
		size = 1
	}
	return a1, size
}

func execWRITE(env *Environment) {
	a1, size := tapeRange(env)
	data := make([]uint8, size)
	for i := range data {
		data[i] = env.mem.Peek(a1 + uint16(i))
	}
	env.log(fmt.Sprintf("WRITE(A1=%04X, %v bytes)", a1, size))
	env.con.TapeWrite(data)
}

func execREAD(env *Environment) {
	a1, size := tapeRange(env)
	data := env.con.TapeRead(size)
	env.log(fmt.Sprintf("READ(A1=%04X, %v bytes) => %v bytes", a1, size, len(data)))
	n := len(data)
	if n > size {
		n = size
	}
	for i := 0; i < n; i++ {
		env.mem.Poke(a1+uint16(i), data[i])
	}
	if len(data) != size {
		// The checksum of a real tape would not match
		env.printText("ERR\a\n")
	}
}

// printText sends host generated text to the console and mirrors it
// to the text page, as if the ROM had printed it through COUT.
func (env *Environment) printText(s string) {
	for _, ch := range []uint8(s) {
		switch ch {
		case '\n':
			env.con.Write("\n")
			env.textPageNewLine()
			env.setColumn(0)
		case 0x07:
			env.con.Write("\a")
		default:
			env.con.Write(string(rune(ch)))
			env.textPagePutChar(ch)
			env.setColumn(env.col + 1)
		}
	}
}
