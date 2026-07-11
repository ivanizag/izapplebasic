package main

import (
	"fmt"

	"github.com/ivanizag/iz6502"
)

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
}

func newEnvironment(rom []uint8, con console) (*environment, error) {
	var env environment
	mem, err := newAppleMemory(rom)
	if err != nil {
		return nil, err
	}
	env.mem = mem
	env.cpu = iz6502.NewNMOS6502(mem)
	env.con = con
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
