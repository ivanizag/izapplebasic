package izapplebasic

import "strings"

// consoleMock is a Console for the tests. It takes the input from a
// list of lines and collects the output, echoing the input lines to
// build a transcript similar to what a terminal would show.
type consoleMock struct {
	linesIn []string
	lineIn  int
	pending []uint8 // chars buffered for ReadChar()
	output  string

	// onReadLine, if set, is called before each line is returned,
	// with the line about to be served. Used to inject events at a
	// precise point of the session.
	onReadLine func(line string)

	// metaFn, if set, implements MetaCommand as a frontend would
	metaFn func(line string) bool
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
	c.Write(prompt)
	line, eof := c.nextLine()
	if eof {
		return "", true
	}
	c.Write(line + "\n")
	return line, false
}

func (c *consoleMock) ReadChar() (uint8, bool) {
	for len(c.pending) == 0 {
		line, eof := c.nextLine()
		if eof {
			return 0, true
		}
		c.pending = append([]uint8(line), '\r')
	}
	ch := c.pending[0]
	c.pending = c.pending[1:]
	return ch, false
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
