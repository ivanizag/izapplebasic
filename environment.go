package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ivanizag/iz6502"
)

const controlCDelayToQuitMs = 500

type environment struct {
	cpu         *iz6502.State
	mem         *appleMemory
	con         console
	col         uint8 // column position on the host output
	stop        bool
	apiLog      bool
	apiLogIO    bool
	clearScreen bool   // clear the host screen on HOME
	maxCycles   uint64 // stop after this many cycles, 0 for no limit

	lastEscape time.Time
}

/*
escape processes a control-C press: two in fast succession quit,
a single one presents a control-C keypress to the emulated machine
to break the running BASIC program.

It is called from the signal handler goroutine, and directly by the
liner console when editing a line, as liner has the terminal in raw
mode and control-C does not raise a signal there.
*/
func (env *environment) escape() {
	timestamp := time.Now()
	delay := timestamp.Sub(env.lastEscape)
	if delay.Milliseconds() < controlCDelayToQuitMs {
		// Two control-c in fast succession, quit
		env.con.close()
		fmt.Println()
		os.Exit(0)
	}
	env.lastEscape = timestamp
	env.mem.breakPending.Store(true)
}

// newEnvironment builds the machine. The console is not set, it has
// to be assigned to env.con before calling run.
func newEnvironment(rom []uint8) (*environment, error) {
	var env environment
	mem, err := newAppleMemory(rom)
	if err != nil {
		return nil, err
	}
	env.mem = mem
	env.cpu = iz6502.NewNMOS6502(mem)
	patchMonitorTraps(mem)
	env.cpu.Reset()
	return &env, nil
}

func (env *environment) log(msg string) {
	if env.apiLog {
		fmt.Printf("[[[%s]]]\n", msg)
	}
}

func (env *environment) logIO(msg string) {
	if env.apiLogIO {
		fmt.Printf("[[[%s]]]\n", msg)
	}
}
