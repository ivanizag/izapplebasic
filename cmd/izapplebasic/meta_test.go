package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	iz "github.com/ivanizag/izapplebasic"
)

// testConsole feeds lines and collects output, with the meta
// commands of the command line frontend.
type testConsole struct {
	env     *iz.Environment
	tape    *tapeDrive
	linesIn []string
	lineIn  int
	output  string
}

func (c *testConsole) ReadLine(prompt string) (string, bool) {
	c.output += prompt
	if c.lineIn >= len(c.linesIn) {
		return "", true
	}
	line := c.linesIn[c.lineIn]
	c.lineIn++
	c.output += line + "\n"
	return line, false
}

func (c *testConsole) ReadChar() (uint8, bool) { return 0, true }
func (c *testConsole) Write(s string)          { c.output += s }
func (c *testConsole) Clear()                  {}
func (c *testConsole) close()                  {}

func (c *testConsole) MetaCommand(line string) bool {
	return metaCommand(c.env, c, c.tape, line)
}

func (c *testConsole) TapeWrite(data []uint8) {
	c.tape.write(c, data)
}

func (c *testConsole) TapeRead(requested int) []uint8 {
	return c.tape.read(c, requested)
}

func runConsole(t *testing.T, linesIn []string) (string, *iz.Environment) {
	t.Helper()
	con := &testConsole{linesIn: linesIn, tape: newTapeDrive(t.TempDir())}
	env, err := iz.NewEnvironment(nil)
	if err != nil {
		t.Fatal(err)
	}
	con.env = env
	env.SetConsole(con)
	env.MaxCycles = 200_000_000
	env.Run()
	return con.output, env
}

func assertContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Errorf("output does not contain %q:\n%s", want, output)
	}
}

func TestMetaSaveAndLoad(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "test.state")

	out, _ := runConsole(t, []string{
		"X=21",
		"/save " + filename,
	})
	assertContains(t, out, "state saved to "+filename)

	out, _ = runConsole(t, []string{
		"/load " + filename,
		"PRINT X*2",
	})
	assertContains(t, out, "state loaded from "+filename)
	assertContains(t, out, "42")
}

func TestMetaScreenshot(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "test.png")
	out, _ := runConsole(t, []string{
		"GR",
		"COLOR=9",
		"PLOT 5,5",
		"/screenshot " + filename,
	})
	assertContains(t, out, "GR-MIX40 screenshot saved to "+filename)

	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
		t.Errorf("empty image: %v", img.Bounds())
	}
}

func TestMetaQuit(t *testing.T) {
	out, _ := runConsole(t, []string{
		"/quit",
		"PRINT 123",
	})
	if strings.Contains(out, "123") {
		t.Errorf("no input must be processed after /quit:\n%s", out)
	}
}

func TestMetaHelp(t *testing.T) {
	out, _ := runConsole(t, []string{"/help"})
	assertContains(t, out, "/screenshot")
	assertContains(t, out, "/save")
}

func TestMetaTape(t *testing.T) {
	out, _ := runConsole(t, []string{
		"/tape game",
		`10 PRINT "TAPED"`,
		"SAVE",
		"/rewind",
		"NEW",
		"LOAD",
		"RUN",
		"/tape",
	})
	assertContains(t, out, "tape game inserted and rewound")
	assertContains(t, out, "tape game at block 0")
	assertContains(t, out, "TAPED")
	assertContains(t, out, "tape game at block 2")
	// The block operations are silent unless tracing
	if strings.Contains(out, "wrote block") {
		t.Errorf("the block operations must be silent:\n%s", out)
	}
}

func TestMetaTapeInvalidName(t *testing.T) {
	out, _ := runConsole(t, []string{"/tape ../evil"})
	assertContains(t, out, "invalid tape name")
}

func TestMetaUnknown(t *testing.T) {
	out, _ := runConsole(t, []string{
		"/nosuchcommand",
		"PRINT 7*6",
	})
	assertContains(t, out, "unknown meta command /nosuchcommand")
	assertContains(t, out, "42")
}
