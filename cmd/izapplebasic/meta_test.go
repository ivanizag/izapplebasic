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
	if c.lineIn >= len(c.linesIn) {
		return "", true
	}
	line := c.linesIn[c.lineIn]
	c.lineIn++
	c.output += line + "\n"
	return line, false
}

func (c *testConsole) ReadKeys(prompt string) (string, bool) { return c.ReadLine(prompt) }
func (c *testConsole) Write(s string)                        { c.output += s }
func (c *testConsole) Clear()                                {}
func (c *testConsole) close()                                {}

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

func TestMetaExport(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "test.bas")
	out, _ := runConsole(t, []string{
		`10 PRINT "A"`,
		"20 A=1:PRINT A",
		"/export " + filename,
	})
	assertContains(t, out, "2 lines exported to "+filename)

	program, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	want := "10  PRINT \"A\"\n20 A = 1: PRINT A\n"
	if string(program) != want {
		t.Errorf("unexpected file:\ngot:\n%s\nwant:\n%s", program, want)
	}
}

func TestMetaExportNoProgram(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "empty.bas")
	out, _ := runConsole(t, []string{"/export " + filename})
	assertContains(t, out, "there is no program to export")
	if _, err := os.Stat(filename); err == nil {
		t.Error("no file must be written when there is no program")
	}
}

func TestCompleteCommandName(t *testing.T) {
	tape := newTapeDrive(t.TempDir())
	// A "/" alone offers all of them, to be listed on a second tab
	if _, got, _ := completeMeta(tape, "/", 1); len(got) != len(metaCommands) {
		t.Errorf("all the commands expected, got %v", got)
	}
	// A prefix narrows it down, keeping the ones sharing it
	assertCompletes(t, tape, "/ta", "", []string{"/tape ", "/tapes "})
	// An unambiguous prefix completes to the command and its space
	assertCompletes(t, tape, "/scr", "", []string{"/screenshot "})
	// The ones taking no argument complete to just the command
	assertCompletes(t, tape, "/qu", "", []string{"/quit"})
	// The shell escape glues to its command, no space
	assertCompletes(t, tape, "/!", "", []string{"/!"})
	// Typed in uppercase it still completes, the dispatch lowercases
	assertCompletes(t, tape, "/EXP", "", []string{"/export "})

	assertCompletes(t, tape, "/nosuch", "", nil)
	// A line for the emulated machine is not a meta command
	assertCompletes(t, tape, "PRINT 2+2", "PRINT 2+2", nil)
}

func TestCompleteLanguageArgument(t *testing.T) {
	tape := newTapeDrive(t.TempDir())
	assertCompletes(t, tape, "/reset ", "/reset ", []string{"applesoft", "integer"})
	assertCompletes(t, tape, "/reset in", "/reset ", []string{"integer"})
	assertCompletes(t, tape, "/reset IN", "/reset ", []string{"integer"})
	assertCompletes(t, tape, "/reset pas", "/reset ", nil)
	// A command with nothing to offer for its argument
	assertCompletes(t, tape, "/rewind ", "/rewind ", nil)
}

func TestCompleteTapeArgument(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"tape-adventure-00.tape", "tape-adventure-01.tape", "tape-notes-00.tape"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte{1}, 0644); err != nil {
			t.Fatal(err)
		}
	}
	tape := newTapeDrive(dir)
	assertCompletes(t, tape, "/tape ", "/tape ", []string{"adventure", "notes"})
	assertCompletes(t, tape, "/tape ad", "/tape ", []string{"adventure"})
	assertCompletes(t, tape, "/tape zz", "/tape ", nil)
}

func TestCompleteFileArgument(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "game.bas"), []byte("10 END\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "game.state"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "tapes"), 0755); err != nil {
		t.Fatal(err)
	}
	tape := newTapeDrive(dir)

	// The folder of the prefix is listed, the folders keep a
	// separator so the name can go on being typed
	assertCompletes(t, tape, "/load "+dir+string(filepath.Separator), "/load ", []string{
		filepath.Join(dir, "game.bas"),
		filepath.Join(dir, "game.state"),
		filepath.Join(dir, "tapes") + string(filepath.Separator),
	})
	// /tapes only offers the folders
	assertCompletes(t, tape, "/tapes "+dir+string(filepath.Separator), "/tapes ", []string{
		filepath.Join(dir, "tapes") + string(filepath.Separator),
	})
	// The hidden entries only when the prefix asks for them
	assertCompletes(t, tape, "/load "+filepath.Join(dir, ".h"), "/load ", []string{
		filepath.Join(dir, ".hidden"),
	})
	// A folder that is not there completes to nothing
	assertCompletes(t, tape, "/load /nosuchfolder/x", "/load ", nil)
}

// What is after the cursor is kept, only the word being typed is
// completed
func TestCompleteKeepsTheTail(t *testing.T) {
	tape := newTapeDrive(t.TempDir())
	head, got, tail := completeMeta(tape, "/reset integer", len("/reset in"))
	if head != "/reset " || tail != "teger" {
		t.Errorf("got head %q and tail %q", head, tail)
	}
	if len(got) != 1 || got[0] != "integer" {
		t.Errorf("got %v", got)
	}
}

func assertCompletes(t *testing.T, tape *tapeDrive, line string, wantHead string, want []string) {
	t.Helper()
	head, got, _ := completeMeta(tape, line, len(line))
	if head != wantHead {
		t.Errorf("completing %q: got head %q, want %q", line, head, wantHead)
	}
	if len(got) != len(want) {
		t.Errorf("completing %q: got %v, want %v", line, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("completing %q: got %v, want %v", line, got, want)
			return
		}
	}
}

// The table drives the help and the completion, it has to hold every
// command that is really dispatched
func TestMetaCommandsAreAllDispatched(t *testing.T) {
	for _, cmd := range metaCommands {
		if cmd.name == "/quit" {
			// It stops the machine, the rest of the lines are lost
			continue
		}
		line := cmd.name
		if cmd.name == "/!" {
			line = "/!true"
		}
		out, _ := runConsole(t, []string{line})
		if strings.Contains(out, "unknown meta command") {
			t.Errorf("%s is on the help but not dispatched:\n%s", cmd.name, out)
		}
	}
}

func TestMetaReset(t *testing.T) {
	out, env := runConsole(t, []string{
		"X=42",
		"/reset",
		"PRINT X",
	})
	assertContains(t, out, "the machine has been reset")
	// X is gone, the line after the reset reaches the new machine
	assertContains(t, out, "\n0\n")
	if env.Language() != iz.LanguageApplesoft {
		t.Error("a reset with no argument must stay on the same BASIC")
	}
}

func TestMetaResetToInteger(t *testing.T) {
	out, env := runConsole(t, []string{
		"/reset integer",
		"PRINT 2+2",
	})
	assertContains(t, out, "the machine has been reset to Integer BASIC")
	assertContains(t, out, ">PRINT 2+2")
	assertContains(t, out, "4")
	if env.Language() != iz.LanguageInteger {
		t.Error("the machine must be running Integer BASIC")
	}
}

func TestMetaResetUnknownLanguage(t *testing.T) {
	out, env := runConsole(t, []string{"/reset pascal"})
	assertContains(t, out, "unknown BASIC")
	if env.Language() != iz.LanguageApplesoft {
		t.Error("an unknown name must leave the machine alone")
	}
}

func TestMetaShell(t *testing.T) {
	out, _ := runConsole(t, []string{"/!echo hello from the host"})
	assertContains(t, out, "hello from the host")
}

// The rest of the line keeps its case, its words and its quoting,
// and goes through a shell so the pipes and the globs work
func TestMetaShellIsNotParsed(t *testing.T) {
	out, _ := runConsole(t, []string{`/!echo "Mixed Case" | tr a-z A-Z`})
	assertContains(t, out, "MIXED CASE")
}

func TestMetaShellFailure(t *testing.T) {
	out, _ := runConsole(t, []string{"/!exit 3"})
	assertContains(t, out, "exit status 3")
}

func TestMetaShellEmpty(t *testing.T) {
	out, _ := runConsole(t, []string{"/!"})
	assertContains(t, out, "usage: /!<command>")
}

// A shell escape must not reach the emulated machine as BASIC
func TestMetaShellDoesNotReachBasic(t *testing.T) {
	out, _ := runConsole(t, []string{"/!echo ok", "PRINT 2+2"})
	assertContains(t, out, "ok")
	assertContains(t, out, "4")
	if strings.Contains(out, "SYNTAX") {
		t.Errorf("the shell escape must not reach Applesoft:\n%s", out)
	}
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

func TestMetaTapes(t *testing.T) {
	out, _ := runConsole(t, []string{
		"/tape game",
		`10 PRINT "TAPED"`,
		"SAVE",
		"/tape notes",
		"SAVE",
		"/tapes",
	})
	// An Applesoft SAVE writes the length header and the program
	assertContains(t, out, "  game (2 blocks)\n")
	assertContains(t, out, "  notes (2 blocks, inserted)\n")
}

func TestMetaTapesGivenFolder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"tape-other-00.tape", "tape-other-01.tape", "tape-other-02.tape"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte{1, 2}, 0644); err != nil {
			t.Fatal(err)
		}
	}
	out, _ := runConsole(t, []string{"/tapes " + dir})
	assertContains(t, out, "tapes in "+dir)
	// The tape of the drive is elsewhere, nothing is inserted here
	assertContains(t, out, "  other (3 blocks)\n")
	if strings.Contains(out, "inserted") {
		t.Errorf("no tape of this folder is inserted:\n%s", out)
	}
}

func TestMetaTapesEmpty(t *testing.T) {
	out, _ := runConsole(t, []string{"/tapes " + t.TempDir()})
	assertContains(t, out, "no tapes in ")
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
