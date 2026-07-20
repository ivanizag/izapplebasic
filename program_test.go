package izapplebasic

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

// The program used to exercise the interesting cases at once:
// quoted strings, the ":" separator, REM text, the comparison and
// arithmetic tokens and the ones past 0xc8 that are operators
// instead of statements.
var exportProgramLines = []string{
	`10 PRINT "A"`,
	"20 A=1:PRINT A",
	`30 IF A>1 THEN PRINT "X"`,
	"40 FOR I=1 TO 3: NEXT I",
	"50 REM A COMMENT",
	"60 B = INT(RND(1)*10)+A/2",
	`70 PRINT LEFT$("HELLO",2);MID$("WORLD",1,3)`,
}

func TestExportFormat(t *testing.T) {
	env, _ := testEnvironment(t, exportProgramLines[:5])
	env.Run()
	got, err := env.ExportProgram()
	if err != nil {
		t.Fatal(err)
	}
	want := `10  PRINT "A"
20 A = 1: PRINT A
30  IF A > 1 THEN  PRINT "X"
40  FOR I = 1 TO 3: NEXT I
50  REM  A COMMENT
`
	if got != want {
		t.Errorf("unexpected listing:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

/*
The ROM is the oracle: the program the exporter returns has to be
the one LIST prints. Only the content is compared, the whitespace is
dropped on both sides: LIST wraps on the 40 columns of the screen,
continuing on the next one, while the exporter returns whole lines.
The exact spacing is pinned by TestExportFormat instead.
*/
func TestExportMatchesList(t *testing.T) {
	env, con := testEnvironment(t, append(exportProgramLines, "LIST"))
	env.Run()
	got, err := env.ExportProgram()
	if err != nil {
		t.Fatal(err)
	}

	// The LIST output is the transcript after the last typed line
	_, listed, found := strings.Cut(con.output, "LIST\n")
	if !found {
		t.Fatalf("no LIST output:\n%s", con.output)
	}
	want := listedLines(listed)
	exported := listedLines(got)
	if len(exported) != len(want) {
		t.Fatalf("%v lines exported, LIST printed %v:\n%s", len(exported), len(want), listed)
	}
	for i := range want {
		if exported[i] != want[i] {
			t.Errorf("line %v differs:\n exported %q\n LIST     %q", i, exported[i], want[i])
		}
	}
}

var notSpace = regexp.MustCompile(`\s+`)

/*
listedLines splits a listing into its logical lines with all the
whitespace removed. A line of the screen starts a logical one when
it starts with a line number, and continues the previous one when it
starts with a space: that is where LIST wrapped it. Anything else,
like the prompt closing the transcript, is not part of the listing.
*/
func listedLines(s string) []string {
	var lines []string
	for _, raw := range strings.Split(s, "\n") {
		line := notSpace.ReplaceAllString(raw, "")
		if line == "" {
			continue
		}
		switch {
		case raw[0] >= '0' && raw[0] <= '9':
			lines = append(lines, line)
		case raw[0] == ' ' && len(lines) > 0:
			lines[len(lines)-1] += line
		}
	}
	return lines
}

func TestExportEmptyProgram(t *testing.T) {
	env, _ := testEnvironment(t, nil)
	env.Run()
	got, err := env.ExportProgram()
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("no program expected, got %q", got)
	}
}

func TestExportNotSupportedOnInteger(t *testing.T) {
	env, _ := testEnvironmentWithLanguage(t, LanguageInteger, []string{`10 PRINT "A"`})
	env.Run()
	_, err := env.ExportProgram()
	if !errors.Is(err, ErrExportNotSupported) {
		t.Errorf("ErrExportNotSupported expected, got %v", err)
	}
}

func TestExportTokenNames(t *testing.T) {
	env, _ := testEnvironment(t, nil)
	names := env.applesoftTokenNames()
	// The tokens of the Apple II+ ROM, 0x80 to 0xea
	if len(names) != 107 {
		t.Errorf("107 token names expected, got %v", len(names))
	}
	if names[0] != "END" {
		t.Errorf("the first token must be END, got %q", names[0])
	}
	if last := names[len(names)-1]; last != "MID$" {
		t.Errorf("the last token must be MID$, got %q", last)
	}
}

// A chain of lines that does not move forward must be reported, not
// followed until the end of time
func TestExportMalformedProgram(t *testing.T) {
	env, _ := testEnvironment(t, []string{`10 PRINT "A"`, `20 PRINT "B"`})
	env.Run()
	start := env.peekWord(zpTXTTAB)
	// Point the first line back at itself
	env.mem.Poke(start, uint8(start&0xff))
	env.mem.Poke(start+1, uint8(start>>8))
	if _, err := env.ExportProgram(); err == nil {
		t.Error("a program that does not end must be reported")
	}
}
