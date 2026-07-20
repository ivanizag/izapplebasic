package izapplebasic

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
	addrGETLN1 = uint16(0xfd6f) // Read a line into 0x200
	addrCOUT1  = uint16(0xfdf0) // Output the char in A to the screen
	addrWRITE  = uint16(0xfecd) // Write the range A1..A2 to the cassette
	addrREAD   = uint16(0xfefd) // Read from the cassette into A1..A2

	/*
		MON, the monitor entry point reached at the end of the reset
		code, once the screen and the I/O vectors are initialized and
		before the bell and the "*" prompt. Used to enter a BASIC
		that the reset vector does not reach on its own.

		It is not a trap, no RTS is patched there: the run loop only
		moves the PC away on the boot. CALL -151 enters the monitor
		at MONZ (0xff69), further along, so it never collides.
	*/
	addrMON = uint16(0xff65)
)

// Monitor zero page usage
const (
	zpCH  = uint16(0x24) // Cursor horizontal position
	zpCV  = uint16(0x25) // Cursor vertical position
	zpA1L = uint16(0x3c) // Range start for the monitor commands
	zpA2L = uint16(0x3e) // Range end, inclusive

	inputBuffer     = uint16(0x0200)
	inputBufferSize = 255
)

/*
Only GETLN1 is intercepted for the line input: the other two entry
points, GETLNZ (0xfd67) and GETLN (0xfd6a), are real ROM code that
prints the CR and the prompt through the intercepted COUT and falls
into GETLN1. This way the prompt reaches the output and the text
page with no special handling.
*/
var trapAddresses = []uint16{
	addrHOME,
	addrKEYIN,
	addrGETLN1,
	addrCOUT1,
	addrWRITE,
	addrREAD,
}

func patchMonitorTraps(mem *appleMemory) {
	for _, address := range trapAddresses {
		mem.pokeROM(address, 0x60 /* RTS */)
	}
}

// Run is the main emulation loop. It executes instructions and
// intercepts the monitor calls, until there is no more input or
// Stop is called.
func (env *Environment) Run() {
	for !env.stop {
		/*
			The traps are processed before executing the instruction
			at the entry point, the patched RTS. This way a state
			restored while the machine was waiting on GETLN resumes
			by serving that GETLN.
		*/
		pc, _ := env.cpu.GetPCAndSP()
		if env.pendingColdStart && pc == addrMON {
			/*
				The original monitor has finished the reset: the
				screen and the I/O vectors are set up and nothing has
				been printed yet, the bell and the "*" prompt come
				next. Enter the BASIC instead. The PC is moved before
				the instruction runs, the loop dispatches on the new
				one.
			*/
			env.pendingColdStart = false
			env.cpu.SetPC(env.language.info().coldStart)
			continue
		}
		switch pc {
		case addrCOUT1:
			execCOUT1(env)
		case addrKEYIN:
			execKEYIN(env)
		case addrGETLN1:
			execGETLN(env)
		case addrWRITE:
			execWRITE(env)
		case addrREAD:
			execREAD(env)
		case addrHOME:
			env.log("HOME()")
			if env.col > 0 {
				// The cursor moves to the start of a line
				env.con.Write("\n")
			}
			env.con.Clear()
			// textPageClear also homes the cursor row CV
			env.textPageClear()
			env.setColumn(0)
		}
		if env.stop {
			break
		}

		env.cpu.ExecuteInstruction()
		if env.BreakCycles != 0 && env.cpu.GetCycles() >= env.BreakCycles {
			// The program has been running for too long, break it
			env.mem.breakPending.Store(true)
		}
		if env.MaxCycles != 0 && env.cpu.GetCycles() >= env.MaxCycles {
			env.stop = true
		}
	}
}

func execCOUT1(env *Environment) {
	a, _, _, _ := env.cpu.GetAXYP()
	ch := a & 0x7f
	if env.TraceMonitorIO {
		if ch >= 0x20 && ch < 0x7f {
			env.logIO(fmt.Sprintf("COUT1($%02X '%c')", a, ch))
		} else {
			env.logIO(fmt.Sprintf("COUT1($%02X)", a))
		}
	}
	switch {
	case ch == '\r':
		env.con.Write("\n")
		env.textPageNewLine()
		env.setColumn(0)
	case ch == 0x07:
		env.con.Write("\a")
	case ch == 0x08:
		env.con.Write("\b")
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
			env.con.Write(" ")
			env.textPagePutChar(' ')
			env.col++
		}
		env.con.Write(string(rune(ch)))
		env.textPagePutChar(ch)
		env.setColumn(env.col + 1)
	default:
		// Other control chars are ignored
	}
}

// setColumn keeps the host column and the monitor cursor position
// CH in sync. Applesoft uses CH for TAB() and the print zones.
func (env *Environment) setColumn(col uint8) {
	env.col = col
	env.mem.Poke(zpCH, col)
}

/*
readInput asks the frontend for a line and gives it the chance to
process it as a meta command, taking more lines until one is for the
machine.

The second value reports that the wait this was serving no longer
exists, the caller has to return without delivering anything: the
input ended, a meta command stopped the machine, or one moved the
CPU with a reset or a state load. The run loop then dispatches on
whatever the new PC is.
*/
func (env *Environment) readInput(keys bool) (string, bool) {
	/*
		The prompt was printed by the caller through COUT: the "]"
		of Applesoft, the ">" of Integer BASIC, the "*" of the
		monitor, or the text of an INPUT. It is the current line of
		the screen, reconstructed from the text page so it survives
		a state save and load.
	*/
	prompt := env.currentLine()
	entryPC, _ := env.cpu.GetPCAndSP()
	for {
		var line string
		var eof bool
		if keys {
			line, eof = env.con.ReadKeys(prompt)
		} else {
			line, eof = env.con.ReadLine(prompt)
		}
		if eof {
			env.stop = true
			return "", true
		}
		if !env.con.MetaCommand(line) {
			return line, false
		}
		if env.stop {
			// A meta command stopped the machine
			return "", true
		}
		if pc, _ := env.cpu.GetPCAndSP(); pc != entryPC {
			return "", true
		}
	}
}

/*
execKEYIN serves the keystroke reads. Integer BASIC reads its direct
mode this way, one key at a time instead of calling GETLN, so the
line is taken whole from the frontend and handed over key by key.
The meta commands are processed on the line boundary, the only point
where the machine is not in the middle of a line.
*/
func execKEYIN(env *Environment) {
	env.log("KEYIN()")
	_, x, y, p := env.cpu.GetAXYP()
	if env.mem.breakPending.Swap(false) {
		// Deliver the pending control-C, Applesoft consumes it
		// with RDKEY when breaking a running program
		env.cpu.SetAXYP(0x83, x, y, p)
		return
	}
	if len(env.pendingIn) == 0 {
		line, abandoned := env.readInput(true)
		if abandoned {
			return
		}
		env.pendingIn = append([]uint8(line), '\r')
	}
	ch := env.pendingIn[0]
	env.pendingIn = env.pendingIn[1:]
	if env.Uppercase && ch >= 'a' && ch <= 'z' {
		ch -= 'a' - 'A'
	}
	env.cpu.SetAXYP(ch|0x80, x, y, p)
}

func execGETLN(env *Environment) {
	line, abandoned := env.readInput(false)
	if abandoned {
		return
	}
	// A control-C pressed while typing would have been consumed as
	// input by the real GETLN, it must not break the next RUN
	env.mem.breakPending.Store(false)
	if env.Uppercase {
		line = strings.ToUpper(line)
	}
	env.log(fmt.Sprintf("GETLN() => %q", line))

	// Mirror the typed line to the text page, the real GETLN echoes
	// it through COUT. The prompt is already there.
	for _, ch := range []uint8(line) {
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
