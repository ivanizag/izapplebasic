package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// consoleStdio is the command line frontend. It reads lines from
// stdin and writes to stdout.
type consoleStdio struct {
	in        *bufio.Reader
	pending   []uint8 // chars buffered for readChar()
	uppercase bool
}

func newConsoleStdio(uppercase bool) *consoleStdio {
	var c consoleStdio
	c.in = bufio.NewReader(os.Stdin)
	c.uppercase = uppercase
	return &c
}

func (c *consoleStdio) readLine(prompt string) (string, bool) {
	fmt.Print(prompt)
	line, err := c.in.ReadString('\n')
	if err != nil && line == "" {
		return "", true
	}
	line = strings.TrimRight(line, "\r\n")
	if c.uppercase {
		line = strings.ToUpper(line)
	}
	return line, false
}

func (c *consoleStdio) readChar() (uint8, bool) {
	for len(c.pending) == 0 {
		line, eof := c.readLine("")
		if eof {
			return 0, true
		}
		c.pending = append([]uint8(line), '\r')
	}
	ch := c.pending[0]
	c.pending = c.pending[1:]
	return ch, false
}

func (c *consoleStdio) write(s string) {
	fmt.Print(s)
}

func (c *consoleStdio) clear() {
	fmt.Print("\033[2J\033[H")
}

func (c *consoleStdio) close() {}
