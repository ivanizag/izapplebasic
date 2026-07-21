package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	iz "github.com/ivanizag/izapplebasic"
)

/*
tapeDrive emulates the cassette deck with files: the block written
or read at position n of the tape NAME is the file
"tape-NAME-nn.tape", holding the raw bytes of the record. Both
pointers, the tape name and the position, live on the frontend.
*/
type tapeDrive struct {
	dir   string
	name  string
	pos   int
	trace bool // show the block operations, silent otherwise
}

var validTapeName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,30}$`)

func newTapeDrive(dir string) *tapeDrive {
	return &tapeDrive{dir: dir, name: "default"}
}

func (t *tapeDrive) blockFilename(pos int) string {
	return filepath.Join(t.dir, fmt.Sprintf("tape-%s-%02d.tape", t.name, pos))
}

// log reports the block operations, only when tracing the monitor
// calls with -m or -M, in the same style as the core traces.
func (t *tapeDrive) log(format string, a ...interface{}) {
	if t.trace {
		fmt.Printf("[[["+format+"]]]\n", a...)
	}
}

func (t *tapeDrive) write(con iz.Console, data []uint8) {
	err := os.WriteFile(t.blockFilename(t.pos), data, 0644)
	if err != nil {
		con.Write(fmt.Sprintf("error writing the tape: %v\n", err))
		return
	}
	t.log("wrote block %v of tape %s (%v bytes)", t.pos, t.name, len(data))
	t.pos++
}

func (t *tapeDrive) read(con iz.Console, requested int) []uint8 {
	data, err := os.ReadFile(t.blockFilename(t.pos))
	if err != nil {
		// The core shows the monitor ERR
		t.log("end of tape %s at block %v", t.name, t.pos)
		return nil
	}
	t.log("read block %v of tape %s (%v bytes)", t.pos, t.name, len(data))
	t.pos++
	return data
}

/*
tapes returns the tape names recorded on a folder, sorted, with the
number of blocks of each one. A tape is the set of the block files
"tape-NAME-nn.tape" sharing a name.
*/
func tapes(dir string) ([]string, map[string]int) {
	matches, _ := filepath.Glob(filepath.Join(dir, "tape-*.tape"))
	blocks := make(map[string]int)
	for _, match := range matches {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(match), "tape-"), ".tape")
		// Trim the block number to get the name of the tape
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

// metaCommand processes the /tape, /tapes and /rewind commands,
// shared by the consoles. Returns false if the line is not a tape
// command.
func (t *tapeDrive) metaCommand(con iz.Console, command string, arg string) bool {
	switch command {
	case "/tape":
		if arg == "" {
			con.Write(fmt.Sprintf("tape %s at block %v\n", t.name, t.pos))
			return true
		}
		if !validTapeName.MatchString(arg) {
			con.Write("invalid tape name, use up to 30 letters, digits, - or _\n")
			return true
		}
		t.name = arg
		t.pos = 0
		con.Write(fmt.Sprintf("tape %s inserted and rewound\n", t.name))

	case "/tapes":
		dir := t.dir
		if arg != "" {
			dir = arg
		}
		names, blocks := tapes(dir)
		if len(names) == 0 {
			con.Write(fmt.Sprintf("no tapes in %s, record one with the BASIC SAVE\n", dir))
			return true
		}
		con.Write(fmt.Sprintf("tapes in %s:\n", dir))
		for _, name := range names {
			inserted := ""
			// The inserted tape is only the one of this drive
			if dir == t.dir && name == t.name {
				inserted = ", inserted"
			}
			con.Write(fmt.Sprintf("  %s (%v blocks%s)\n", name, blocks[name], inserted))
		}

	case "/rewind":
		pos := 0
		if arg != "" {
			if _, err := fmt.Sscanf(arg, "%d", &pos); err != nil || pos < 0 {
				con.Write("invalid block number\n")
				return true
			}
		}
		t.pos = pos
		con.Write(fmt.Sprintf("tape %s at block %v\n", t.name, t.pos))

	default:
		return false
	}
	return true
}
