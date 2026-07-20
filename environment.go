package izapplebasic

import (
	"fmt"

	"github.com/ivanizag/iz6502"
)

// Environment is an Apple II running Applesoft or Integer BASIC
// with the console I/O intercepted.
type Environment struct {
	cpu *iz6502.State
	mem *appleMemory
	con Console
	col uint8 // column position on the host output

	// language is the BASIC in ROM. It travels with the state: a
	// state saved on one BASIC restores its ROM on load.
	language Language

	// pendingColdStart makes the run loop enter the BASIC once the
	// monitor reset code has initialized the machine. See boot.
	pendingColdStart bool

	// pendingIn holds the rest of an input line taken from the
	// frontend, to be served key by key. See execKEYIN.
	pendingIn []uint8

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
// embedded Apple II+ ROM when nil. A ROM given here is booted as
// the Apple II+ one, through the reset vector. The console has to
// be set with SetConsole before calling Run.
func NewEnvironment(rom []uint8) (*Environment, error) {
	if rom == nil {
		return NewEnvironmentWithLanguage(LanguageApplesoft)
	}
	return newEnvironment(rom, LanguageApplesoft)
}

// NewEnvironmentWithLanguage builds the machine with the embedded
// ROM of the given BASIC.
func NewEnvironmentWithLanguage(language Language) (*Environment, error) {
	return newEnvironment(language.info().data, language)
}

func newEnvironment(rom []uint8, language Language) (*Environment, error) {
	var env Environment
	mem, err := newAppleMemory(rom, embeddedCharGen)
	if err != nil {
		return nil, err
	}
	env.mem = mem
	env.language = language
	env.Uppercase = true
	env.cpu = iz6502.NewNMOS6502(mem)
	patchMonitorTraps(mem)
	env.boot()
	return &env, nil
}

/*
boot restarts the CPU on the reset vector. On the Apple II+ that is
the autostart monitor, which lands on Applesoft by itself. On the
Apple II it is the original monitor, which initializes the screen
and the I/O vectors and then stops on the "*" prompt: the run loop
takes over there and enters Integer BASIC on its cold start.
*/
func (env *Environment) boot() {
	env.cpu.Reset()
	env.pendingColdStart = env.language.info().coldStart != 0
}

// Language returns the BASIC in ROM.
func (env *Environment) Language() Language {
	return env.language
}

// setLanguage swaps the ROM in place, keeping the RAM and the CPU
// untouched. The traps have to be patched again, the ROM bytes
// holding them were just overwritten.
func (env *Environment) setLanguage(language Language) error {
	if err := env.mem.loadROM(language.info().data); err != nil {
		return err
	}
	patchMonitorTraps(env.mem)
	env.language = language
	return nil
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
// and the CPU restarts on the reset vector. The BASIC in ROM is
// kept, use ResetWithLanguage to change it.
func (env *Environment) Reset() {
	for i := range env.mem.data[:ioAreaStart] {
		env.mem.data[i] = 0
	}
	env.mem.textMode = true
	env.mem.mixedMode = false
	env.mem.page2 = false
	env.mem.hiResMode = false
	env.mem.breakPending.Store(false)
	env.boot()
	env.col = 0
	// The keys not yet read belong to the machine that is gone
	env.pendingIn = nil
}

// ResetWithLanguage swaps the ROM for the one of the given BASIC
// and does a cold boot on it.
func (env *Environment) ResetWithLanguage(language Language) error {
	if err := env.setLanguage(language); err != nil {
		return err
	}
	env.Reset()
	return nil
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
