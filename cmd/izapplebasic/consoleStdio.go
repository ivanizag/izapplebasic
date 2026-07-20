package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	iz "github.com/ivanizag/izapplebasic"
)

// consoleStdio is the plain command line frontend. It reads lines
// from stdin and writes to stdout.
type consoleStdio struct {
	env         *iz.Environment
	tape        *tapeDrive
	in          *bufio.Reader
	clearScreen bool
}

func newConsoleStdio(env *iz.Environment, tape *tapeDrive, clearScreen bool) *consoleStdio {
	var c consoleStdio
	c.env = env
	c.tape = tape
	c.in = bufio.NewReader(os.Stdin)
	c.clearScreen = clearScreen
	return &c
}

func (c *consoleStdio) ReadLine(prompt string) (string, bool) {
	// The prompt was already printed through Write
	line, err := c.in.ReadString('\n')
	if err != nil && line == "" {
		return "", true
	}
	line = strings.TrimRight(line, "\r\n")
	return line, false
}

// ReadKeys is the same read: this console never echoes the input,
// the prompt and the typed text are already on the terminal.
func (c *consoleStdio) ReadKeys(prompt string) (string, bool) {
	return c.ReadLine(prompt)
}

func (c *consoleStdio) Write(s string) {
	fmt.Print(s)
}

func (c *consoleStdio) Clear() {
	if c.clearScreen {
		fmt.Print("\033[2J\033[H")
	}
}

func (c *consoleStdio) MetaCommand(line string) bool {
	return metaCommand(c.env, c, c.tape, line)
}

func (c *consoleStdio) TapeWrite(data []uint8) {
	c.tape.write(c, data)
}

func (c *consoleStdio) TapeRead(requested int) []uint8 {
	return c.tape.read(c, requested)
}

func (c *consoleStdio) close() {}
