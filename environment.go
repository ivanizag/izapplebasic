package izapplebasic

import (
	"fmt"

	"github.com/ivanizag/iz6502"
)

// Environment is an Apple II+ running Applesoft BASIC with the
// console I/O intercepted.
type Environment struct {
	cpu *iz6502.State
	mem *appleMemory
	con Console
	col uint8 // column position on the host output

	// promptShown tracks if the prompt of the GETLN currently
	// waiting is already mirrored on the text page
	promptShown bool

	stop bool

	// Uppercase converts the input to uppercase, as the Apple II+
	// keyboard would
	Uppercase bool

	// TraceMonitor dumps the intercepted monitor calls, and
	// TraceMonitorIO also the char output ones
	TraceMonitor   bool
	TraceMonitorIO bool

	// MaxCycles stops the emulation after this many cycles, 0 for
	// no limit
	MaxCycles uint64

	// BreakCycles injects a control-C when reached, to break BASIC
	// programs running for too long. 0 for no limit.
	BreakCycles uint64
}

// NewEnvironment builds the machine with the given ROM, or the
// embedded Apple II+ ROM when nil. The console has to be set with
// SetConsole before calling Run.
func NewEnvironment(rom []uint8) (*Environment, error) {
	if rom == nil {
		rom = embeddedROM
	}
	var env Environment
	mem, err := newAppleMemory(rom, embeddedCharGen)
	if err != nil {
		return nil, err
	}
	env.mem = mem
	env.Uppercase = true
	env.cpu = iz6502.NewNMOS6502(mem)
	patchMonitorTraps(mem)
	env.cpu.Reset()
	return &env, nil
}

// SetConsole attaches the frontend.
func (env *Environment) SetConsole(con Console) {
	env.con = con
}

// Stop makes Run return at the next instruction.
func (env *Environment) Stop() {
	env.stop = true
}

// Break presents a control-C keypress to the emulated machine, to
// break a running BASIC program. Safe to call from any goroutine.
func (env *Environment) Break() {
	env.mem.breakPending.Store(true)
}

// Reset takes the machine back to a cold boot: the RAM is cleared
// and the CPU restarts on the reset vector.
func (env *Environment) Reset() {
	for i := range env.mem.data[:ioAreaStart] {
		env.mem.data[i] = 0
	}
	env.mem.textMode = true
	env.mem.mixedMode = false
	env.mem.page2 = false
	env.mem.hiResMode = false
	env.mem.breakPending.Store(false)
	env.cpu.Reset()
	env.col = 0
	env.promptShown = false
}

// Cycles returns the CPU cycles executed so far.
func (env *Environment) Cycles() uint64 {
	return env.cpu.GetCycles()
}

// SetTraceCPU dumps the CPU execution operations.
func (env *Environment) SetTraceCPU(trace bool) {
	env.cpu.SetTrace(trace)
}

// SetTraceIO dumps the accesses to the softswitches at 0xc0xx.
func (env *Environment) SetTraceIO(trace bool) {
	env.mem.traceIO = trace
}

// GraphicsDirty reports if the emulated code drew graphics since
// the last call to ClearGraphicsDirty.
func (env *Environment) GraphicsDirty() bool {
	return env.mem.graphicsDirty
}

func (env *Environment) ClearGraphicsDirty() {
	env.mem.graphicsDirty = false
}

func (env *Environment) log(msg string) {
	if env.TraceMonitor {
		fmt.Printf("[[[%s]]]\n", msg)
	}
}

func (env *Environment) logIO(msg string) {
	if env.TraceMonitorIO {
		fmt.Printf("[[[%s]]]\n", msg)
	}
}
