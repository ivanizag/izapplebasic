package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

/*
Cassette deck of the telegram frontend. The blocks are files
"tape-NAME-nn.tape" on the folder of the user, holding the raw bytes
of each record. The inserted tape and its position are persisted in
the tapePointer file, the emulation is stateless between messages.
*/

const (
	defaultTapeName = "default"
	maxTapeBlocks   = 30 // in total per user, across all the tapes

	tapePointerFilename = "tape.txt"
)

func (c *consoleTelegram) blockFilename(pos int) string {
	return filepath.Join(c.dir, fmt.Sprintf("tape-%s-%02d.tape", c.tapeName, pos))
}

// loadTapePointer restores the inserted tape and position, called
// before running the message.
func (c *consoleTelegram) loadTapePointer() {
	c.tapeName = defaultTapeName
	c.tapePos = 0
	data, err := os.ReadFile(filepath.Join(c.dir, tapePointerFilename))
	if err != nil {
		return
	}
	var name string
	var pos int
	if _, err := fmt.Sscanf(string(data), "%s %d", &name, &pos); err != nil {
		return
	}
	if validStateName.MatchString(name) && pos >= 0 {
		c.tapeName = name
		c.tapePos = pos
	}
}

// saveTapePointer persists the inserted tape and position, called
// after running the message.
func (c *consoleTelegram) saveTapePointer() {
	content := fmt.Sprintf("%s %d\n", c.tapeName, c.tapePos)
	os.WriteFile(filepath.Join(c.dir, tapePointerFilename), []byte(content), 0644)
}

func (c *consoleTelegram) countTapeBlocks() int {
	matches, _ := filepath.Glob(filepath.Join(c.dir, "tape-*.tape"))
	return len(matches)
}

func (c *consoleTelegram) TapeWrite(data []uint8) {
	filename := c.blockFilename(c.tapePos)
	if _, err := os.Stat(filename); err != nil &&
		c.countTapeBlocks() >= maxTapeBlocks {
		// Overwriting is fine, adding one more block is not
		c.notice(fmt.Sprintf("you already have %v tape blocks, /deletetape one first\n",
			maxTapeBlocks))
		return
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		c.notice("error writing the tape\n")
		return
	}
	c.notice(fmt.Sprintf("wrote block %v of tape %s\n", c.tapePos, c.tapeName))
	c.tapePos++
}

func (c *consoleTelegram) TapeRead(requested int) []uint8 {
	data, err := os.ReadFile(c.blockFilename(c.tapePos))
	if err != nil {
		c.notice(fmt.Sprintf("end of tape %s at block %v, see /tape and /rewind\n",
			c.tapeName, c.tapePos))
		return nil
	}
	c.notice(fmt.Sprintf("read block %v of tape %s\n", c.tapePos, c.tapeName))
	c.tapePos++
	return data
}

// tapes returns the tape names present on the folder of the user,
// with their block counts.
func (c *consoleTelegram) tapes() ([]string, map[string]int) {
	matches, _ := filepath.Glob(filepath.Join(c.dir, "tape-*.tape"))
	blocks := make(map[string]int)
	for _, match := range matches {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(match), "tape-"), ".tape")
		if i := strings.LastIndexByte(name, '-'); i > 0 {
			blocks[name[:i]]++
		}
	}
	names := make([]string, 0, len(blocks))
	for name := range blocks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, blocks
}

func (c *consoleTelegram) metaTape(arg string) {
	if arg == "" {
		c.notice(fmt.Sprintf("tape %s at block %v\n", c.tapeName, c.tapePos))
		return
	}
	if !validStateName.MatchString(arg) {
		c.notice("invalid name, use up to 30 letters, digits, - or _\n")
		return
	}
	c.tapeName = arg
	c.tapePos = 0
	c.notice(fmt.Sprintf("tape %s inserted and rewound\n", c.tapeName))
}

func (c *consoleTelegram) metaRewind(arg string) {
	pos := 0
	if arg != "" {
		if _, err := fmt.Sscanf(arg, "%d", &pos); err != nil || pos < 0 {
			c.notice("invalid block number\n")
			return
		}
	}
	c.tapePos = pos
	c.notice(fmt.Sprintf("tape %s at block %v\n", c.tapeName, c.tapePos))
}

func (c *consoleTelegram) metaDeleteTape(arg string) {
	if arg == "" {
		c.notice("use /deletetape <name>, see /list\n")
		return
	}
	if !validStateName.MatchString(arg) {
		c.notice("invalid name, use up to 30 letters, digits, - or _\n")
		return
	}
	matches, _ := filepath.Glob(filepath.Join(c.dir, "tape-"+arg+"-*.tape"))
	if len(matches) == 0 {
		c.notice("no tape named " + arg + ", see /list\n")
		return
	}
	for _, match := range matches {
		os.Remove(match)
	}
	c.notice(fmt.Sprintf("tape %s deleted, %v blocks\n", arg, len(matches)))
}
