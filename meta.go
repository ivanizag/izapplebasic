package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/ivanizag/izapple2/screen"
)

/*
Meta commands are input lines starting with "/". They are processed
on the host and never reach the emulated machine. They work on any
frontend.
*/

const defaultStateFilename = "izapplebasic.state"

// metaCommand processes the line if it is a meta command, returning
// false when the line is regular input for the emulated machine.
func (env *environment) metaCommand(line string) bool {
	if !strings.HasPrefix(line, "/") {
		return false
	}
	fields := strings.Fields(line)
	command := strings.ToLower(fields[0])
	switch command {
	case "/help":
		env.con.write("meta commands:\n")
		env.con.write("  /help\n")
		env.con.write("  /quit: exit\n")
		env.con.write("  /save [filename]: save the emulation state\n")
		env.con.write("  /load [filename]: load the emulation state\n")
		env.con.write("  /screenshot [filename.png]: save an image of the emulated screen\n")

	case "/quit":
		env.stop = true

	case "/save":
		filename := defaultStateFilename
		if len(fields) > 1 {
			filename = fields[1]
		}
		err := env.saveState(filename)
		if err != nil {
			env.con.write(fmt.Sprintf("error saving the state: %v\n", err))
		} else {
			env.con.write(fmt.Sprintf("state saved to %s\n", filename))
		}

	case "/load":
		filename := defaultStateFilename
		if len(fields) > 1 {
			filename = fields[1]
		}
		err := env.loadState(filename)
		if err != nil {
			env.con.write(fmt.Sprintf("error loading the state: %v\n", err))
		} else {
			env.con.write(fmt.Sprintf("state loaded from %s\n", filename))
		}

	case "/screenshot":
		filename := time.Now().Format("screenshot-20060102-150405.png")
		if len(fields) > 1 {
			filename = fields[1]
		}
		err := screen.SaveSnapshot(env.mem, screen.ScreenModeNTSC, filename)
		if err != nil {
			env.con.write(fmt.Sprintf("error saving the screenshot: %v\n", err))
		} else {
			env.con.write(fmt.Sprintf("%s screenshot saved to %s\n",
				screen.VideoModeName(env.mem), filename))
		}

	default:
		env.con.write(fmt.Sprintf("unknown meta command %s, try /help\n", command))
	}
	return true
}
