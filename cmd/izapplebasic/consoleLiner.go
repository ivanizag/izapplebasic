package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	iz "github.com/ivanizag/izapplebasic"
	"github.com/peterh/liner"
)

const historyFilename = ".izapplebasichistory"

// consoleLiner is the command line frontend with readline like
// editing: up and down arrows recall previous commands.
type consoleLiner struct {
	env         *iz.Environment
	esc         *escaper
	liner       *liner.State
	pendingOut  string  // unfinished output line, shown as the prompt
	pendingIn   []uint8 // chars buffered for ReadChar()
	clearScreen bool
}

func newConsoleLiner(env *iz.Environment, esc *escaper, clearScreen bool) *consoleLiner {
	var c consoleLiner
	c.env = env
	c.esc = esc
	c.clearScreen = clearScreen
	c.liner = liner.NewLiner()
	c.liner.SetCtrlCAborts(true)
	if f, err := os.Open(historyFilename); err == nil {
		c.liner.ReadHistory(f)
		f.Close()
	}
	return &c
}

func (c *consoleLiner) ReadLine(prompt string) (string, bool) {
	/*
		Text pending on the current line, like the prompts that
		Applesoft sends char by char with COUT, has to be handled by
		liner to be able to redraw the line while editing.
	*/
	fmt.Print("\r")
	line, err := c.liner.Prompt(c.pendingOut + prompt)
	if errors.Is(err, liner.ErrInvalidPrompt) {
		fmt.Println()
		line, err = c.liner.Prompt("")
	}
	c.pendingOut = ""
	if errors.Is(err, liner.ErrPromptAborted) {
		/*
			Control-C while editing: liner has the terminal in raw
			mode and no signal is raised, process the escape here to
			keep the double press to quit working.
		*/
		c.esc.escape()
		return "", false
	}
	if errors.Is(err, io.EOF) {
		// Control-D is ignored, exit with two control-C
		return "", false
	}
	if err != nil {
		panic(err)
	}
	if line != "" {
		c.liner.AppendHistory(line)
	}
	return line, false
}

func (c *consoleLiner) ReadChar() (uint8, bool) {
	for len(c.pendingIn) == 0 {
		line, eof := c.ReadLine("")
		if eof {
			return 0, true
		}
		c.pendingIn = append([]uint8(line), '\r')
	}
	ch := c.pendingIn[0]
	c.pendingIn = c.pendingIn[1:]
	return ch, false
}

func (c *consoleLiner) Write(s string) {
	fmt.Print(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		c.pendingOut = s[i+1:]
	} else {
		c.pendingOut += s
	}
}

func (c *consoleLiner) Clear() {
	if c.clearScreen {
		fmt.Print("\033[2J\033[H")
		c.pendingOut = ""
	}
}

func (c *consoleLiner) MetaCommand(line string) bool {
	return metaCommand(c.env, c, line)
}

func (c *consoleLiner) close() {
	if f, err := os.Create(historyFilename); err == nil {
		c.liner.WriteHistory(f)
		f.Close()
	}
	c.liner.Close()
}
