package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"time"

	iz "github.com/ivanizag/izapplebasic"
)

/*
Meta commands of the command line frontend. They are input lines
starting with "/", processed on the host and never reaching the
emulated machine. All the filesystem access happens here, the
emulation core has none.
*/

const defaultStateFilename = "izapplebasic.state"

func metaCommand(env *iz.Environment, con iz.Console, tape *tapeDrive, line string) bool {
	if !strings.HasPrefix(line, "/") {
		return false
	}
	fields := strings.Fields(line)
	command := strings.ToLower(fields[0])
	arg := ""
	if len(fields) > 1 {
		arg = fields[1]
	}
	if tape.metaCommand(con, command, arg) {
		return true
	}
	switch command {
	case "/help":
		con.Write("meta commands:\n")
		con.Write("  /help\n")
		con.Write("  /quit: exit\n")
		con.Write("  /save [filename]: save the emulation state\n")
		con.Write("  /load [filename]: load the emulation state\n")
		con.Write("  /screenshot [filename.png]: save an image of the emulated screen\n")
		con.Write("  /tape [name]: insert a cassette tape for SAVE and LOAD, or show it\n")
		con.Write("  /rewind [block]: move the tape to a block, 0 by default\n")

	case "/quit":
		env.Stop()

	case "/save":
		filename := defaultStateFilename
		if len(fields) > 1 {
			filename = fields[1]
		}
		err := saveStateFile(env, filename)
		if err != nil {
			con.Write(fmt.Sprintf("error saving the state: %v\n", err))
		} else {
			con.Write(fmt.Sprintf("state saved to %s\n", filename))
		}

	case "/load":
		filename := defaultStateFilename
		if len(fields) > 1 {
			filename = fields[1]
		}
		err := loadStateFile(env, filename)
		if err != nil {
			con.Write(fmt.Sprintf("error loading the state: %v\n", err))
		} else {
			con.Write(fmt.Sprintf("state loaded from %s\n", filename))
		}

	case "/screenshot":
		filename := time.Now().Format("screenshot-20060102-150405.png")
		if len(fields) > 1 {
			filename = fields[1]
		}
		writeImage(con, env.Snapshot(), filename, env.VideoModeName())

	default:
		con.Write(fmt.Sprintf("unknown meta command %s, try /help\n", command))
	}
	return true
}

func saveStateFile(env *iz.Environment, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return env.SaveState(f)
}

func loadStateFile(env *iz.Environment, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return env.LoadState(f)
}

// writeImage saves a snapshot as a PNG file and reports the result.
func writeImage(con iz.Console, img image.Image, filename string, description string) {
	f, err := os.Create(filename)
	if err != nil {
		con.Write(fmt.Sprintf("error saving the screenshot: %v\n", err))
		return
	}
	defer f.Close()
	err = png.Encode(f, img)
	if err != nil {
		con.Write(fmt.Sprintf("error saving the screenshot: %v\n", err))
		return
	}
	con.Write(fmt.Sprintf("%s screenshot saved to %s\n", description, filename))
}
