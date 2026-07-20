package izapplebasic

import "strings"

// consoleMock is a Console for the tests. It takes the input from a
// list of lines and collects the output, echoing the input lines to
// build a transcript similar to what a terminal would show.
type consoleMock struct {
	linesIn []string
	lineIn  int
	output  string

	// onReadLine, if set, is called before each line is returned,
	// with the line about to be served. Used to inject events at a
	// precise point of the session.
	onReadLine func(line string)

	// metaFn, if set, implements MetaCommand as a frontend would
	metaFn func(line string) bool

	// An in-memory tape deck
	tapeBlocks [][]uint8
	tapePos    int
}

func newConsoleMock(linesIn []string) *consoleMock {
	return &consoleMock{linesIn: linesIn}
}

func (c *consoleMock) nextLine() (string, bool) {
	if c.lineIn >= len(c.linesIn) {
		return "", true
	}
	line := c.linesIn[c.lineIn]
	c.lineIn++
	if c.onReadLine != nil {
		c.onReadLine(line)
	}
	return line, false
}

func (c *consoleMock) ReadLine(prompt string) (string, bool) {
	// The prompt already arrived through Write, echo only the line
	line, eof := c.nextLine()
	if eof {
		return "", true
	}
	c.Write(line + "\n")
	return line, false
}

// ReadKeys does not echo, the machine prints the keys it reads
func (c *consoleMock) ReadKeys(prompt string) (string, bool) {
	return c.nextLine()
}

func (c *consoleMock) Write(s string) {
	c.output += s
}

func (c *consoleMock) Clear() {
	c.output += "\f"
}

func (c *consoleMock) MetaCommand(line string) bool {
	if c.metaFn != nil && strings.HasPrefix(line, "/") {
		return c.metaFn(line)
	}
	return false
}

func (c *consoleMock) TapeWrite(data []uint8) {
	block := append([]uint8{}, data...)
	if c.tapePos < len(c.tapeBlocks) {
		c.tapeBlocks[c.tapePos] = block
	} else {
		c.tapeBlocks = append(c.tapeBlocks, block)
	}
	c.tapePos++
}

func (c *consoleMock) TapeRead(requested int) []uint8 {
	if c.tapePos >= len(c.tapeBlocks) {
		return nil
	}
	block := c.tapeBlocks[c.tapePos]
	c.tapePos++
	return block
}
