package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
const defaultProgramFilename = "program.bas"

// argKind tells what the argument of a meta command completes to.
type argKind int

const (
	argNone     argKind = iota // Nothing to offer, like a block number
	argLanguage                // A BASIC of the emulated machine
	argTapeName                // A tape recorded on the folder of the drive
	argFile                    // A file of the host
	argFolder                  // A folder of the host
)

/*
metaCommands is the list of meta commands, used for the /help text
and to complete them when the line is typed. usage is the name with
its argument, "/!" glues to its command and the others take it as a
separate word.
*/
var metaCommands = []struct {
	name        string
	usage       string
	description string
	arg         argKind
}{
	{"/help", "/help", "list the meta commands", argNone},
	{"/quit", "/quit", "exit", argNone},
	{"/reset", "/reset [applesoft|integer]", "reboot, losing everything not saved", argLanguage},
	{"/save", "/save [filename]", "save the emulation state", argFile},
	{"/load", "/load [filename]", "load the emulation state", argFile},
	{"/screenshot", "/screenshot [filename.png]", "save an image of the emulated screen", argFile},
	{"/export", "/export [filename.bas]", "save the program as text, Applesoft only", argFile},
	{"/tape", "/tape [name]", "insert a cassette tape for SAVE and LOAD, or show it", argTapeName},
	{"/tapes", "/tapes [folder]", "list the tapes recorded on a folder", argFolder},
	{"/rewind", "/rewind [block]", "move the tape to a block, 0 by default", argNone},
	{"/!", "/!<command>", "run a command on the host, like /!ls", argNone},
}

/*
completeMeta completes the meta command being typed, for the tab
completion of the readline like console. Before the space it offers
the commands, after it the arguments each one takes: the BASICs of
/reset, the tapes of the drive, the files and folders of the host.

The result is what the console expects: the part of the line kept as
it is, the candidates for the word being typed, and the rest after
the cursor.
*/
func completeMeta(tape *tapeDrive, line string, pos int) (string, []string, string) {
	if pos > len(line) {
		pos = len(line)
	}
	typed, tail := line[:pos], line[pos:]
	if !strings.HasPrefix(typed, "/") {
		// A line for the emulated machine, nothing to complete
		return typed, nil, tail
	}
	name, arg, hasArg := strings.Cut(typed, " ")
	if !hasArg {
		// Still on the command itself, it replaces the whole line
		return "", completeCommandName(typed), tail
	}
	return name + " ", completeArgument(tape, strings.ToLower(name), arg), tail
}

func completeCommandName(typed string) []string {
	var candidates []string
	for _, cmd := range metaCommands {
		if !strings.HasPrefix(cmd.name, strings.ToLower(typed)) {
			continue
		}
		completion := cmd.name
		if cmd.usage != cmd.name && cmd.name != "/!" {
			// It takes an argument as a separate word
			completion += " "
		}
		candidates = append(candidates, completion)
	}
	return candidates
}

func completeArgument(tape *tapeDrive, name string, arg string) []string {
	for _, cmd := range metaCommands {
		if cmd.name != name {
			continue
		}
		switch cmd.arg {
		case argLanguage:
			return matchingNames([]string{"applesoft", "integer"}, arg)
		case argTapeName:
			names, _ := tapes(tape.dir)
			return matchingNames(names, arg)
		case argFile:
			return completePath(arg, false)
		case argFolder:
			return completePath(arg, true)
		}
		return nil
	}
	return nil
}

// matchingNames keeps the candidates starting with the prefix, taken
// without case: the names are all lowercase or a safe charset.
func matchingNames(candidates []string, prefix string) []string {
	var out []string
	for _, candidate := range candidates {
		if strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(prefix)) {
			out = append(out, candidate)
		}
	}
	return out
}

/*
completePath completes a name of the host as a shell would: the
folder already typed is listed and the entries following the rest
are offered, the folders with a separator to go on typing. The
hidden entries only show when the prefix asks for them.
*/
func completePath(prefix string, foldersOnly bool) []string {
	dir, name := filepath.Split(prefix)
	listing := dir
	if listing == "" {
		listing = "."
	}
	entries, err := os.ReadDir(listing)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		if foldersOnly && !entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), name) {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") && !strings.HasPrefix(name, ".") {
			continue
		}
		completion := dir + entry.Name()
		if entry.IsDir() {
			completion += string(filepath.Separator)
		}
		out = append(out, completion)
	}
	sort.Strings(out)
	return out
}

func metaCommand(env *iz.Environment, con iz.Console, tape *tapeDrive, line string) bool {
	if !strings.HasPrefix(line, "/") {
		return false
	}
	if shell, found := strings.CutPrefix(line, "/!"); found {
		// Checked before splitting, the rest of the line is a
		// command for the host with its own words and quoting
		runShellCommand(con, strings.TrimSpace(shell))
		return true
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
		for _, cmd := range metaCommands {
			con.Write(fmt.Sprintf("  %s: %s\n", cmd.usage, cmd.description))
		}

	case "/quit":
		env.Stop()

	case "/reset":
		metaReset(con, env, arg)

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

	case "/export":
		filename := defaultProgramFilename
		if len(fields) > 1 {
			filename = fields[1]
		}
		writeProgram(con, env, filename)

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

/*
metaReset reboots the machine. Without an argument it stays on the
BASIC in use, the name of a BASIC switches the ROM.

The reset moves the CPU, which is what tells the core that the input
wait this meta command was serving no longer exists: the line typed
after it is served by the machine that just booted.
*/
func metaReset(con iz.Console, env *iz.Environment, arg string) {
	if arg == "" {
		env.Reset()
		con.Write("the machine has been reset\n")
		return
	}
	language, ok := iz.ParseLanguage(arg)
	if !ok {
		con.Write("unknown BASIC, use /reset applesoft or /reset integer\n")
		return
	}
	if err := env.ResetWithLanguage(language); err != nil {
		con.Write(fmt.Sprintf("the machine could not be reset: %v\n", err))
		return
	}
	con.Write(fmt.Sprintf("the machine has been reset to %s\n", language.Name()))
}

/*
runShellCommand runs a command on the host and shows its output,
the "/!ls" escape to the shell of the command line frontend.

It is handed to a shell so the pipes, the globs and the quoting work
as they would on the terminal. There is no input: the standard input
of the process is the one being typed on, the emulated machine reads
from it.

This is only on the command line frontend, where the user already
has a shell. The telegram bot has its own meta commands and none of
them reaches this.
*/
func runShellCommand(con iz.Console, command string) {
	if command == "" {
		con.Write("usage: /!<command>, for example /!ls\n")
		return
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	output, err := exec.Command(shell, "-c", command).CombinedOutput()
	if len(output) != 0 {
		text := string(output)
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		con.Write(text)
	}
	if err != nil {
		con.Write(fmt.Sprintf("%s: %v\n", command, err))
	}
}

// writeProgram saves the BASIC program as a text file and reports
// the result.
func writeProgram(con iz.Console, env *iz.Environment, filename string) {
	program, err := env.ExportProgram()
	if err != nil {
		con.Write(fmt.Sprintf("error exporting the program: %v\n", err))
		return
	}
	if program == "" {
		con.Write("there is no program to export\n")
		return
	}
	if err := os.WriteFile(filename, []byte(program), 0644); err != nil {
		con.Write(fmt.Sprintf("error exporting the program: %v\n", err))
		return
	}
	con.Write(fmt.Sprintf("%v lines exported to %s\n",
		strings.Count(program, "\n"), filename))
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
