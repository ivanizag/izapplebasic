package main

import (
	"fmt"
	"strings"
)

/*
Monitor entry points intercepted. See the Apple II Reference Manual
or the AutoF8ROM.dis65 disassembly.

The ROM is patched with an RTS on those addresses. The main loop
detects when the PC reaches one of them, does the work on the host
side and lets the CPU execute the RTS to return to the caller.
*/
const (
	addrHOME   = uint16(0xfc58) // Clear the screen
	addrKEYIN  = uint16(0xfd1b) // Read a key, result in A with the high bit set
	addrGETLNZ = uint16(0xfd67) // CR, then prompt, then read a line into 0x200
	addrGETLN  = uint16(0xfd6a) // Prompt, then read a line into 0x200
	addrGETLN1 = uint16(0xfd6f) // Read a line into 0x200, no prompt
	addrCOUT1  = uint16(0xfdf0) // Output the char in A to the screen
)

// Monitor zero page usage
const (
	zpCH     = uint16(0x24) // Cursor horizontal position
	zpCV     = uint16(0x25) // Cursor vertical position
	zpPROMPT = uint16(0x33) // Prompt character for GETLN

	inputBuffer     = uint16(0x0200)
	inputBufferSize = 255
)

var trapAddresses = []uint16{
	addrHOME,
	addrKEYIN,
	addrGETLNZ,
	addrGETLN,
	addrGETLN1,
	addrCOUT1,
}

func patchMonitorTraps(mem *appleMemory) {
	for _, address := range trapAddresses {
		mem.pokeROM(address, 0x60 /* RTS */)
	}
}

// run is the main emulation loop. It executes instructions and
// intercepts the monitor calls.
func run(env *environment) {
	for !env.stop {
		env.cpu.ExecuteInstruction()
		if env.maxCycles != 0 && env.cpu.GetCycles() >= env.maxCycles {
			env.stop = true
		}

		pc, _ := env.cpu.GetPCAndSP()
		switch pc {
		case addrCOUT1:
			execCOUT1(env)
		case addrKEYIN:
			execKEYIN(env)
		case addrGETLNZ:
			execGETLN(env, true, true)
		case addrGETLN:
			execGETLN(env, false, true)
		case addrGETLN1:
			execGETLN(env, false, false)
		case addrHOME:
			env.log("HOME()")
			if env.clearScreen {
				env.con.clear()
			} else if env.col > 0 {
				// Not clearing, but the cursor moves to the start
				// of a line
				env.con.write("\n")
			}
			// textPageClear also homes the cursor row CV
			env.textPageClear()
			env.setColumn(0)
		}
	}
}

func execCOUT1(env *environment) {
	a, _, _, _ := env.cpu.GetAXYP()
	ch := a & 0x7f
	if env.apiLogIO {
		if ch >= 0x20 && ch < 0x7f {
			env.logIO(fmt.Sprintf("COUT1($%02X '%c')", a, ch))
		} else {
			env.logIO(fmt.Sprintf("COUT1($%02X)", a))
		}
	}
	switch {
	case ch == '\r':
		env.con.write("\n")
		env.textPageNewLine()
		env.setColumn(0)
	case ch == 0x07:
		env.con.write("\a")
	case ch == 0x08:
		env.con.write("\b")
		if env.col > 0 {
			env.setColumn(env.col - 1)
		}
	case ch >= 0x20:
		/*
			On a real Apple II the screen is random access, code can
			move the cursor just by changing CH. Applesoft does that
			for the comma separated print zones and for HTAB. On a
			stream we can only move forward, filling with spaces.
		*/
		targetCol := env.mem.Peek(zpCH)
		for env.col < targetCol {
			env.con.write(" ")
			env.textPagePutChar(' ')
			env.col++
		}
		env.con.write(string(rune(ch)))
		env.textPagePutChar(ch)
		env.setColumn(env.col + 1)
	default:
		// Other control chars are ignored
	}
}

// setColumn keeps the host column and the monitor cursor position
// CH in sync. Applesoft uses CH for TAB() and the print zones.
func (env *environment) setColumn(col uint8) {
	env.col = col
	env.mem.Poke(zpCH, col)
}

func execKEYIN(env *environment) {
	env.log("KEYIN()")
	_, x, y, p := env.cpu.GetAXYP()
	if env.mem.breakPending.Swap(false) {
		// Deliver the pending control-C, Applesoft consumes it
		// with RDKEY when breaking a running program
		env.cpu.SetAXYP(0x83, x, y, p)
		return
	}
	ch, eof := env.con.readChar()
	if eof {
		env.stop = true
		return
	}
	if env.uppercase && ch >= 'a' && ch <= 'z' {
		ch -= 'a' - 'A'
	}
	env.cpu.SetAXYP(ch|0x80, x, y, p)
}

func execGETLN(env *environment, crFirst bool, showPrompt bool) {
	if crFirst {
		env.con.write("\n")
		env.textPageNewLine()
		env.setColumn(0)
	}
	prompt := ""
	if showPrompt {
		prompt = string(rune(env.mem.Peek(zpPROMPT) & 0x7f))
	}
	var line string
	for {
		var eof bool
		line, eof = env.con.readLine(prompt)
		if eof {
			env.stop = true
			return
		}
		if !env.metaCommand(line) {
			break
		}
		if env.stop {
			// A meta command like /quit stopped the machine
			return
		}
	}
	// A control-C pressed while typing would have been consumed as
	// input by the real GETLN, it must not break the next RUN
	env.mem.breakPending.Store(false)
	if env.uppercase {
		line = strings.ToUpper(line)
	}
	env.log(fmt.Sprintf("GETLN() => %q", line))

	// Mirror the prompt and the typed line to the text page, the
	// real GETLN echoes them through COUT
	for _, ch := range []uint8(prompt + line) {
		env.textPagePutChar(ch)
		env.col++
	}
	env.textPageNewLine()
	if len(line) > inputBufferSize {
		line = line[:inputBufferSize]
	}
	for i := 0; i < len(line); i++ {
		env.mem.Poke(inputBuffer+uint16(i), line[i]|0x80)
	}
	env.mem.Poke(inputBuffer+uint16(len(line)), '\r'|0x80)
	env.setColumn(0)

	// GETLN returns the line length in X and the CR in A
	_, _, y, p := env.cpu.GetAXYP()
	env.cpu.SetAXYP('\r'|0x80, uint8(len(line)), y, p)
}
