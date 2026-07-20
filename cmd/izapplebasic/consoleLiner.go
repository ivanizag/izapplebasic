package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	iz "github.com/ivanizag/izapplebasic"
	"github.com/peterh/liner"
)

const historyFilename = ".izapplebasichistory"

// consoleLiner is the command line frontend with readline like
// editing: up and down arrows recall previous commands.
type consoleLiner struct {
	env         *iz.Environment
	tape        *tapeDrive
	esc         *escaper
	liner       *liner.State
	clearScreen bool
}

func newConsoleLiner(env *iz.Environment, tape *tapeDrive, esc *escaper, clearScreen bool) *consoleLiner {
	var c consoleLiner
	c.env = env
	c.tape = tape
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
	line, _ := c.prompt(prompt)
	return line, false
}

/*
ReadKeys reads the line the same way, but the machine is going to
print the keys as it reads them, over the line liner already left on
the terminal. Erase that line and put the prompt back on it: the
echo of the machine then completes it, instead of showing it twice.
*/
func (c *consoleLiner) ReadKeys(prompt string) (string, bool) {
	line, typed := c.prompt(prompt)
	if typed {
		// Up to the line liner left, clear it, prompt again
		fmt.Printf("\033[A\r\033[K%s", prompt)
	}
	return line, false
}

/*
prompt reads a line with the editor. The second value is false when
nothing was entered and liner left no line on the terminal: a
control-C escape or a control-D.

The prompt is the text already printed on the current line, like the
"]" of Applesoft or the text of an INPUT: liner redraws it to be
able to edit the line.
*/
func (c *consoleLiner) prompt(prompt string) (string, bool) {
	fmt.Print("\r")
	line, err := c.liner.Prompt(prompt)
	if errors.Is(err, liner.ErrInvalidPrompt) {
		fmt.Println()
		line, err = c.liner.Prompt("")
	}
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
	return line, true
}

func (c *consoleLiner) Write(s string) {
	fmt.Print(s)
}

func (c *consoleLiner) Clear() {
	if c.clearScreen {
		fmt.Print("\033[2J\033[H")
	}
}

func (c *consoleLiner) MetaCommand(line string) bool {
	return metaCommand(c.env, c, c.tape, line)
}

func (c *consoleLiner) TapeWrite(data []uint8) {
	c.tape.write(c, data)
}

func (c *consoleLiner) TapeRead(requested int) []uint8 {
	return c.tape.read(c, requested)
}

func (c *consoleLiner) close() {
	if f, err := os.Create(historyFilename); err == nil {
		c.liner.WriteHistory(f)
		f.Close()
	}
	c.liner.Close()
}
