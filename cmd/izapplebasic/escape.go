package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	iz "github.com/ivanizag/izapplebasic"
)

const controlCDelayToQuitMs = 500

/*
escaper processes the control-C presses: two in fast succession
quit, a single one presents a control-C keypress to the emulated
machine to break the running BASIC program.

It is used from the signal handler goroutine, and directly by the
liner console when editing a line, as liner has the terminal in raw
mode and control-C does not raise a signal there.
*/
type escaper struct {
	env        *iz.Environment
	lastEscape time.Time
	closeFn    func()
}

func newEscaper(env *iz.Environment) *escaper {
	return &escaper{env: env}
}

func (esc *escaper) escape() {
	timestamp := time.Now()
	delay := timestamp.Sub(esc.lastEscape)
	if delay.Milliseconds() < controlCDelayToQuitMs {
		// Two control-c in fast succession, quit
		if esc.closeFn != nil {
			esc.closeFn()
		}
		fmt.Println()
		os.Exit(0)
	}
	esc.lastEscape = timestamp
	esc.env.Break()
}

// handleControlC breaks the running BASIC program on control-C
// instead of killing the process.
func handleControlC(esc *escaper) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for range c {
			esc.escape()
		}
	}()
}
