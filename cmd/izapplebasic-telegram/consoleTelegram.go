package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	iz "github.com/ivanizag/izapplebasic"
)

/*
consoleTelegram serves a single incoming Telegram message: the lines
of the message are the input queue, the output text and the images
are collected to build the reply.

When the input queue is exhausted it reports end of input: the run
loop stops with the machine waiting on GETLN or KEYIN, the state is
saved there and the next message continues the session.
*/
type consoleTelegram struct {
	env      *iz.Environment
	dir      string // the folder of the user, for the saved states
	linesIn  []string
	lineIn   int
	segments []outSegment
	images   []image.Image

	// The cassette deck, persisted in the folder of the user
	// between messages
	tapeName string
	tapePos  int
}

/*
outSegment is a piece of the reply. The output of the emulated
machine is shown monospaced, the frontend notices, like the meta
command responses, as normal text.
*/
type outSegment struct {
	mono bool
	text string
}

func newConsoleTelegram(dir string, text string) *consoleTelegram {
	return &consoleTelegram{
		dir:     dir,
		linesIn: strings.Split(text, "\n"),
	}
}

/*
promptPending tells if the prompt still has to be shown: the machine
printed it through COUT, but only within this message. On a resumed
wait it belongs to a previous reply, and a meta command notice may
have separated it from the input it prompts.
*/
func (c *consoleTelegram) promptPending(prompt string) bool {
	n := len(c.segments)
	return prompt != "" &&
		(n == 0 || !c.segments[n-1].mono ||
			!strings.HasSuffix(c.segments[n-1].text, prompt))
}

func (c *consoleTelegram) ReadLine(prompt string) (string, bool) {
	return c.readLine(prompt, true)
}

// ReadKeys takes the line without echoing it: the machine prints the
// keys itself as it reads them, echoing here would show it twice.
func (c *consoleTelegram) ReadKeys(prompt string) (string, bool) {
	return c.readLine(prompt, false)
}

func (c *consoleTelegram) readLine(prompt string, echo bool) (string, bool) {
	if c.lineIn >= len(c.linesIn) {
		/*
			The machine is left waiting for input. Close the reply
			with the pending prompt and a cursor, so it is clear the
			next message will be taken as input: "]_" on the direct
			mode, "NAME? _" on an INPUT.
		*/
		if c.promptPending(prompt) {
			c.Write(prompt)
		}
		c.Write("_")
		return "", true
	}
	line := c.linesIn[c.lineIn]
	c.lineIn++
	/*
		Echo the machine input with its prompt, uppercased as the
		machine will see it: the reply reads as the transcript a
		real terminal would show. The meta command lines are not
		echoed, they never reach the machine.

		On the keys path only the prompt is written: the machine
		prints the line itself as it reads it, key by key.
	*/
	if !strings.HasPrefix(line, "/") {
		if c.promptPending(prompt) {
			c.Write(prompt)
		}
		if echo {
			shown := line
			if c.env.Uppercase {
				shown = strings.ToUpper(shown)
			}
			c.Write(shown + "\n")
		}
	}
	return line, false
}

// Write collects output of the emulated machine, shown monospaced.
func (c *consoleTelegram) Write(s string) {
	c.append(true, s)
}

// notice collects frontend messages, shown as normal text.
func (c *consoleTelegram) notice(s string) {
	c.append(false, s)
}

func (c *consoleTelegram) append(mono bool, s string) {
	if n := len(c.segments); n > 0 && c.segments[n-1].mono == mono {
		c.segments[n-1].text += s
		return
	}
	c.segments = append(c.segments, outSegment{mono, s})
}

// text returns all the collected output flattened, for the log.
func (c *consoleTelegram) text() string {
	var sb strings.Builder
	for _, segment := range c.segments {
		sb.WriteString(segment.text)
	}
	return sb.String()
}

func (c *consoleTelegram) Clear() {
	// The already sent output cannot be retracted
}

// telegramCommands is the meta command list, also registered on
// Telegram to be suggested when the user presses "/".
var telegramCommands = []struct {
	command     string
	description string
}{
	{"help", "list the commands"},
	{"screenshot", "show an image of the emulated screen"},
	{"reset", "reboot the machine, losing everything not saved: /reset [applesoft|integer]"},
	{"save", "save the state: /save [name]"},
	{"load", "load a saved state: /load [name]"},
	{"list", "list the saved states and tapes"},
	{"delete", "delete a saved state: /delete [name]"},
	{"tape", "insert a cassette tape for SAVE and LOAD: /tape [name]"},
	{"rewind", "move the tape to a block: /rewind [block]"},
	{"deletetape", "delete a whole tape: /deletetape <name>"},
}

/*
Meta commands of the telegram frontend. The named states are files
on the folder of the user, the names are restricted to a safe
charset so no user input can reach other paths of the host.
*/
func (c *consoleTelegram) MetaCommand(line string) bool {
	if !strings.HasPrefix(line, "/") {
		return false
	}
	fields := strings.Fields(line)
	command := strings.ToLower(fields[0][1:])
	// In groups Telegram may send "/command@botname"
	command, _, _ = strings.Cut(command, "@")
	arg := ""
	if len(fields) > 1 {
		arg = fields[1]
	}

	switch command {
	case "help", "start": // /start is the telegram convention
		c.notice(fmt.Sprintf("This is an Apple II running %s, type commands as on the real machine.\n",
			c.env.Language().Name()))
		c.notice("Meta commands:\n")
		for _, cmd := range telegramCommands {
			c.notice(fmt.Sprintf("  /%s: %s\n", cmd.command, cmd.description))
		}

	case "screenshot":
		c.images = append(c.images, c.env.Snapshot())

	case "reset":
		c.metaReset(arg)

	case "save":
		c.metaSave(arg)

	case "load":
		c.metaLoad(arg)

	case "list":
		c.metaList()

	case "delete":
		c.metaDelete(arg)

	case "tape":
		c.metaTape(arg)

	case "rewind":
		c.metaRewind(arg)

	case "deletetape":
		c.metaDeleteTape(arg)

	default:
		c.notice("unknown command /" + command + ", try /help\n")
	}
	return true
}

// metaReset reboots the machine. Without an argument it stays on
// the BASIC in use, the name of a BASIC switches the ROM.
func (c *consoleTelegram) metaReset(arg string) {
	if arg == "" {
		c.env.Reset()
		c.notice("the machine has been reset\n")
		return
	}
	language, ok := iz.ParseLanguage(arg)
	if !ok {
		c.notice("unknown BASIC, use /reset applesoft or /reset integer\n")
		return
	}
	if err := c.env.ResetWithLanguage(language); err != nil {
		c.notice("the machine could not be reset: " + err.Error() + "\n")
		return
	}
	c.notice("the machine has been reset to " + language.Name() + "\n")
}

var validStateName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,30}$`)

const defaultStateName = "default"
const maxSavedStates = 30

func (c *consoleTelegram) savedStates() []string {
	matches, _ := filepath.Glob(filepath.Join(c.dir, "saved-*.state"))
	var names []string
	for _, match := range matches {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(match), "saved-"), ".state")
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// stateFilename builds the file path for a named state, or reports
// the invalid name to the user.
func (c *consoleTelegram) stateFilename(name string) (string, string, bool) {
	if name == "" {
		name = defaultStateName
	}
	if !validStateName.MatchString(name) {
		c.notice("invalid name, use up to 30 letters, digits, - or _\n")
		return "", "", false
	}
	return filepath.Join(c.dir, "saved-"+name+".state"), name, true
}

func (c *consoleTelegram) metaSave(arg string) {
	filename, name, ok := c.stateFilename(arg)
	if !ok {
		return
	}
	if _, err := os.Stat(filename); err != nil &&
		len(c.savedStates()) >= maxSavedStates {
		// Overwriting is fine, adding one more is not
		c.notice(fmt.Sprintf("you already have %v saved states, /delete one first\n",
			maxSavedStates))
		return
	}
	f, err := os.Create(filename)
	if err != nil {
		c.notice("error saving the state\n")
		return
	}
	defer f.Close()
	if err := c.env.SaveState(f); err != nil {
		c.notice("error saving the state\n")
		return
	}
	c.notice("state saved as " + name + "\n")
}

func (c *consoleTelegram) metaLoad(arg string) {
	filename, name, ok := c.stateFilename(arg)
	if !ok {
		return
	}
	f, err := os.Open(filename)
	if err != nil {
		c.notice("no state saved as " + name + ", see /list\n")
		return
	}
	defer f.Close()
	if err := c.env.LoadState(f); err != nil {
		c.notice("error loading the state\n")
		return
	}
	c.notice("state loaded from " + name + "\n")
}

func (c *consoleTelegram) metaList() {
	names := c.savedStates()
	tapeNames, tapeBlocks := c.tapes()
	if len(names) == 0 && len(tapeNames) == 0 {
		c.notice("no saved states or tapes, use /save [name] or the BASIC SAVE\n")
		return
	}
	if len(names) > 0 {
		c.notice("saved states:\n")
		for _, name := range names {
			c.notice("  " + name + "\n")
		}
	}
	if len(tapeNames) > 0 {
		c.notice("tapes:\n")
		for _, name := range tapeNames {
			c.notice(fmt.Sprintf("  %s (%v blocks)\n", name, tapeBlocks[name]))
		}
	}
}

func (c *consoleTelegram) metaDelete(arg string) {
	filename, name, ok := c.stateFilename(arg)
	if !ok {
		return
	}
	if err := os.Remove(filename); err != nil {
		c.notice("no state saved as " + name + ", see /list\n")
		return
	}
	c.notice("state " + name + " deleted\n")
}
