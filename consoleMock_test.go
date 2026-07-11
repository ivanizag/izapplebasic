package main

// consoleMock is a console for the tests. It takes the input from a
// list of lines and collects the output, echoing the input lines to
// build a transcript similar to what a terminal would show.
type consoleMock struct {
	linesIn []string
	lineIn  int
	pending []uint8 // chars buffered for readChar()
	output  string

	// onReadLine, if set, is called before each line is returned,
	// with the line about to be served. Used to inject events at a
	// precise point of the session.
	onReadLine func(line string)
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

func (c *consoleMock) readLine(prompt string) (string, bool) {
	c.write(prompt)
	line, eof := c.nextLine()
	if eof {
		return "", true
	}
	c.write(line + "\n")
	return line, false
}

func (c *consoleMock) readChar() (uint8, bool) {
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

func (c *consoleMock) write(s string) {
	c.output += s
}

func (c *consoleMock) clear() {
	c.output += "\f"
}

func (c *consoleMock) close() {}
