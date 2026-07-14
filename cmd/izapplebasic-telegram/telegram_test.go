package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Errorf("output does not contain %q:\n%s", want, output)
	}
}

func telegramMessage(t *testing.T, tf *telegramFrontend, text string) (string, []image.Image) {
	t.Helper()
	segments, images, err := tf.processMessage(1001, "tester", text)
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for _, segment := range segments {
		output.WriteString(segment.text)
	}
	return output.String(), images
}

// telegramSegments returns the reply pieces with their format.
func telegramSegments(t *testing.T, tf *telegramFrontend, text string) []outSegment {
	t.Helper()
	segments, _, err := tf.processMessage(1001, "tester", text)
	if err != nil {
		t.Fatal(err)
	}
	return segments
}

func TestTelegramOutputKinds(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	// The transcript, input echoed with its prompt plus the
	// emulator output, is monospaced. The frontend notices are not,
	// keeping the order of the exchange.
	segments := telegramSegments(t, tf, "PRINT 1\n/save kinds\nPRINT 2")
	var kinds []bool
	for _, segment := range segments {
		kinds = append(kinds, segment.mono)
	}
	if len(kinds) != 3 || !kinds[0] || kinds[1] || !kinds[2] {
		t.Errorf("mono, notice, mono expected, got %v", kinds)
	}
	assertContains(t, segments[0].text, "]PRINT 1")
	assertContains(t, segments[1].text, "state saved as kinds")
	assertContains(t, segments[2].text, "]PRINT 2")
	// The meta command lines are not echoed
	if strings.Contains(segments[0].text, "/save") {
		t.Errorf("the meta command must not be echoed:\n%s", segments[0].text)
	}

	// The echo shows the input as the machine sees it, uppercased
	segments = telegramSegments(t, tf, "print 3+3")
	assertContains(t, segments[0].text, "]PRINT 3+3")

	// The echoed command is transcript, the help text is a notice
	segments = telegramSegments(t, tf, "/help")
	for _, segment := range segments {
		if segment.mono && strings.Contains(segment.text, "meta commands") {
			t.Errorf("the help must not be monospaced: %q", segment.text)
		}
	}
}

func TestTelegramSession(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	// A whole program pasted in one multiline message
	out, _ := telegramMessage(t, tf, "10 PRINT \"HELLO\"\n20 PRINT 6*7\nRUN")
	assertContains(t, out, "HELLO")
	assertContains(t, out, "42")

	// The state persists between messages
	out, _ = telegramMessage(t, tf, "LIST")
	assertContains(t, out, `10  PRINT "HELLO"`)

	// The state and log files exist
	if _, err := os.Stat(filepath.Join(tf.dataDir, "1001", "state")); err != nil {
		t.Error("the state file must exist")
	}
	log, err := os.ReadFile(filepath.Join(tf.dataDir, "1001", "log.txt"))
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, string(log), "tester")
	assertContains(t, string(log), "< LIST")
	assertContains(t, string(log), `> 10  PRINT "HELLO"`)
}

func TestTelegramPendingInput(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	// The program stops at INPUT, the state is saved waiting there.
	// The reply ends with a cursor showing the machine waits.
	out, _ := telegramMessage(t, tf, "10 INPUT \"NAME? \"; N$\n20 PRINT \"HI \" + N$\nRUN")
	assertContains(t, out, "NAME?")
	if !strings.HasSuffix(out, "_") {
		t.Errorf("the reply must end with the waiting cursor:\n%s", out)
	}

	// The next message is the INPUT answer, and the machine is
	// left back on the prompt
	out, _ = telegramMessage(t, tf, "IVAN")
	assertContains(t, out, "HI IVAN")
	assertContains(t, out, "]_")
}

func TestTelegramRunawayProgram(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	out, _ := telegramMessage(t, tf, "10 GOTO 10\nRUN")
	assertContains(t, out, "BREAK\a IN 10")

	// The session is still usable
	out, _ = telegramMessage(t, tf, "PRINT 123")
	assertContains(t, out, "123")
}

func TestTelegramGraphicsSnapshot(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	// Drawing attaches a snapshot automatically
	_, images := telegramMessage(t, tf, "GR\nCOLOR=9\nPLOT 5,5")
	if len(images) != 1 {
		t.Fatalf("one image expected, got %v", len(images))
	}

	// Text only messages do not attach images, even in GR mode
	_, images = telegramMessage(t, tf, "PRINT 1")
	if len(images) != 0 {
		t.Errorf("no image expected, got %v", len(images))
	}
}

func TestTelegramNoFileAccess(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	out, _ := telegramMessage(t, tf, "/save /tmp/evil")
	assertContains(t, out, "invalid name")
	if _, err := os.Stat("/tmp/evil"); err == nil {
		t.Error("no file must be written")
	}

	// /screenshot works but the filename is ignored, the image is
	// attached to the reply
	_, images := telegramMessage(t, tf, "/screenshot /tmp/evil2")
	if len(images) != 1 {
		t.Errorf("one image expected, got %v", len(images))
	}
	if _, err := os.Stat("/tmp/evil2"); err == nil {
		t.Error("no file must be written")
	}
}

func TestTelegramReset(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	telegramMessage(t, tf, "X=42")
	out, _ := telegramMessage(t, tf, "/reset")
	assertContains(t, out, "the machine has been reset")
	out, _ = telegramMessage(t, tf, "PRINT X")
	assertContains(t, out, "0")
}

func TestTelegramSavedStates(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	telegramMessage(t, tf, "X=42")
	out, _ := telegramMessage(t, tf, "/save game")
	assertContains(t, out, "state saved as game")

	// Also with the default name
	out, _ = telegramMessage(t, tf, "/save")
	assertContains(t, out, "state saved as default")

	out, _ = telegramMessage(t, tf, "/list")
	assertContains(t, out, "game")
	assertContains(t, out, "default")

	telegramMessage(t, tf, "/reset")
	out, _ = telegramMessage(t, tf, "/load game\nPRINT X*2")
	assertContains(t, out, "state loaded from game")
	assertContains(t, out, "84")

	out, _ = telegramMessage(t, tf, "/delete game")
	assertContains(t, out, "state game deleted")
	out, _ = telegramMessage(t, tf, "/list")
	if strings.Contains(out, "game") {
		t.Errorf("game must be deleted:\n%s", out)
	}

	out, _ = telegramMessage(t, tf, "/load game")
	assertContains(t, out, "no state saved as game")
}

func TestTelegramStateNameValidation(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	for _, bad := range []string{"../evil", "a/b", "x.y", strings.Repeat("a", 31)} {
		out, _ := telegramMessage(t, tf, "/save "+bad)
		assertContains(t, out, "invalid name")
	}
	// Nothing was written besides the log and the automatic state
	matches, _ := filepath.Glob(filepath.Join(tf.dataDir, "1001", "saved-*"))
	if len(matches) != 0 {
		t.Errorf("no saved files expected, got %v", matches)
	}
}

func TestTelegramSavedStatesLimit(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	for i := 0; i < maxSavedStates; i++ {
		out, _ := telegramMessage(t, tf, fmt.Sprintf("/save state%02d", i))
		assertContains(t, out, "state saved as")
	}
	out, _ := telegramMessage(t, tf, "/save onemore")
	assertContains(t, out, "/delete one first")

	// Overwriting an existing state is still allowed
	out, _ = telegramMessage(t, tf, "/save state07")
	assertContains(t, out, "state saved as state07")

	// After deleting one there is room again
	telegramMessage(t, tf, "/delete state07")
	out, _ = telegramMessage(t, tf, "/save onemore")
	assertContains(t, out, "state saved as onemore")
}

func TestTelegramCommandWithBotName(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	out, _ := telegramMessage(t, tf, "/help@izapplebasicbot")
	assertContains(t, out, "/screenshot")
}

func TestCleanText(t *testing.T) {
	// The lone bell of a boot must not produce a message
	if got := cleanText("\a\n\n"); got != "" {
		t.Errorf("got %q", got)
	}
	// Bells inside error messages are dropped
	if got := cleanText("?SYNTAX ERROR\a\n"); got != "?SYNTAX ERROR" {
		t.Errorf("got %q", got)
	}
	// The leading spaces of aligned output are kept
	if got := cleanText("\n     X=1\n"); got != "     X=1" {
		t.Errorf("got %q", got)
	}
}

func TestTelegramTape(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	// SAVE on the default tape, LOAD in a later message: the tape
	// pointer persists between messages
	out, _ := telegramMessage(t, tf, "10 PRINT \"TAPED\"\nSAVE")
	assertContains(t, out, "wrote block 0 of tape default")
	assertContains(t, out, "wrote block 1 of tape default")

	out, _ = telegramMessage(t, tf, "/rewind")
	assertContains(t, out, "tape default at block 0")

	out, _ = telegramMessage(t, tf, "NEW\nLOAD\nRUN")
	assertContains(t, out, "read block 0 of tape default")
	assertContains(t, out, "TAPED")

	// A named tape
	out, _ = telegramMessage(t, tf, "/tape demo\nSAVE")
	assertContains(t, out, "tape demo inserted and rewound")
	assertContains(t, out, "wrote block 0 of tape demo")

	out, _ = telegramMessage(t, tf, "/list")
	assertContains(t, out, "default (2 blocks)")
	assertContains(t, out, "demo (2 blocks)")

	out, _ = telegramMessage(t, tf, "/deletetape demo")
	assertContains(t, out, "tape demo deleted, 2 blocks")
	out, _ = telegramMessage(t, tf, "/list")
	if strings.Contains(out, "demo") {
		t.Errorf("demo must be deleted:\n%s", out)
	}
}

func TestTelegramTapeEndOfTape(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	out, _ := telegramMessage(t, tf, "LOAD")
	assertContains(t, out, "end of tape default at block 0")
	assertContains(t, out, "ERR")
}

func TestTelegramTapeBlocksLimit(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	// Each SAVE writes 2 blocks without moving back, filling the
	// tape until the limit
	telegramMessage(t, tf, "10 PRINT 1")
	for i := 0; i < maxTapeBlocks/2; i++ {
		out, _ := telegramMessage(t, tf, "SAVE")
		assertContains(t, out, "wrote block")
	}
	out, _ := telegramMessage(t, tf, "SAVE")
	assertContains(t, out, "/deletetape one first")

	// Overwriting existing blocks is still allowed
	out, _ = telegramMessage(t, tf, "/rewind\nSAVE")
	assertContains(t, out, "wrote block 0 of tape default")
}

func TestTelegramTapeNameValidation(t *testing.T) {
	tf := newTelegramFrontend(t.TempDir())
	tf.cycleBudget = 4_000_000

	out, _ := telegramMessage(t, tf, "/tape ../evil")
	assertContains(t, out, "invalid name")
	out, _ = telegramMessage(t, tf, "/rewind x")
	assertContains(t, out, "invalid block number")
}

func TestChunkString(t *testing.T) {
	chunks := chunkString("aaa\nbbb\nccc", 7)
	if len(chunks) != 2 || chunks[0] != "aaa\nbbb" || chunks[1] != "ccc" {
		t.Errorf("bad chunks: %q", chunks)
	}
	chunks = chunkString("aaaaaaaaaa", 4)
	if len(chunks) != 3 || chunks[0] != "aaaa" || chunks[2] != "aa" {
		t.Errorf("bad chunks: %q", chunks)
	}
	chunks = chunkString("short", 100)
	if len(chunks) != 1 || chunks[0] != "short" {
		t.Errorf("bad chunks: %q", chunks)
	}
}
