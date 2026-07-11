package izapplebasic

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ivanizag/izapple2/screen"
)

func TestVideoModeTracking(t *testing.T) {
	m, _ := newAppleMemory(embeddedROM, embeddedCharGen)
	if m.GetCurrentVideoMode() != screen.VideoText40 {
		t.Error("the initial mode must be text")
	}

	m.Peek(ioGraphics)
	m.Peek(ioHiRes)
	m.Peek(ioFullScrn)
	if m.GetCurrentVideoMode() != screen.VideoHGR {
		t.Error("full screen hires expected")
	}

	m.Peek(ioMixed)
	if m.GetCurrentVideoMode() != screen.VideoHGR|screen.VideoMixText40 {
		t.Error("mixed hires expected")
	}

	m.Peek(ioLoRes)
	m.Peek(ioPage2)
	if m.GetCurrentVideoMode() != screen.VideoGR|screen.VideoMixText40|screen.VideoSecondPage {
		t.Error("mixed lores page 2 expected")
	}

	// The switches also work on writes
	m.Poke(ioText, 0)
	m.Poke(ioPage1, 0)
	if m.GetCurrentVideoMode() != screen.VideoText40 {
		t.Error("text mode expected")
	}
}

func TestVideoMemory(t *testing.T) {
	m, _ := newAppleMemory(embeddedROM, embeddedCharGen)
	m.Poke(0x0400, 0xc1)
	if m.GetTextMemory(false, false)[0] != 0xc1 {
		t.Error("text page 1 expected")
	}
	m.Poke(0x0800, 0xc2)
	if m.GetTextMemory(true, false)[0] != 0xc2 {
		t.Error("text page 2 expected")
	}
	m.Poke(0x2000, 0x7f)
	if m.GetVideoMemory(false, false)[0] != 0x7f {
		t.Error("hires page 1 expected")
	}
	m.Poke(0x4000, 0x55)
	if m.GetVideoMemory(true, false)[0] != 0x55 {
		t.Error("hires page 2 expected")
	}
}

// textPageString extracts the chars of the text page, to check that
// the intercepted output is mirrored there for the snapshots.
func textPageString(m *appleMemory) string {
	var s []byte
	for row := uint8(0); row < textRows; row++ {
		address := textPageRowAddress(row)
		for col := uint16(0); col < textColumns; col++ {
			s = append(s, m.Peek(address+col)&0x7f)
		}
		s = append(s, '\n')
	}
	return string(s)
}

func TestTextPageMirror(t *testing.T) {
	env, _ := testEnvironment(t, []string{`PRINT "SNAPSHOT TEST"`})
	env.Run()
	content := textPageString(env.mem)
	assertContains(t, content, "]PRINT \"SNAPSHOT TEST\"")
	assertContains(t, content, "\nSNAPSHOT TEST ")
}

func TestTextPageScroll(t *testing.T) {
	env, _ := testEnvironment(t, []string{
		"10 FOR I=1 TO 30",
		"20 PRINT I",
		"30 NEXT",
		"RUN",
	})
	env.Run()
	content := textPageString(env.mem)
	// The first lines have scrolled out of the 24 rows
	if strings.Contains(content, "]RUN") {
		t.Errorf("the page must have scrolled:\n%s", content)
	}
	assertContains(t, content, "\n30 ")
}

func TestTextWindowPreservesGraphics(t *testing.T) {
	/*
		In mixed GR mode the text is restricted to the bottom rows.
		The text and the graphics share memory, printing and
		scrolling must not corrupt the graphics rows.
	*/
	env, _ := testEnvironment(t, []string{
		"GR",
		"COLOR=9",
		"VLIN 0,39 AT 0",
		"10 FOR I=1 TO 10",
		`20 PRINT "TEXT LINE"`,
		"30 NEXT",
		"RUN",
	})
	env.Run()
	// The first lores column must still hold the color 9 pattern
	if env.mem.Peek(textPage1Address) != 0x99 {
		t.Errorf("the graphics must not be touched by the text scroll, got %02x",
			env.mem.Peek(textPage1Address))
	}
}

func TestPromptOnTextPageWhileWaiting(t *testing.T) {
	// The machine is waiting for input: a snapshot must show the
	// prompt, as a real Apple II would
	env, _ := testEnvironment(t, nil)
	env.Run()
	content := textPageString(env.mem)
	assertContains(t, content, "\n]")
}

func TestPromptNotDuplicatedOnResume(t *testing.T) {
	env1, _ := testEnvironment(t, nil)
	env1.Run()
	var buf bytes.Buffer
	if err := env1.SaveState(&buf); err != nil {
		t.Fatal(err)
	}

	// The restored session serves the same GETLN wait, the prompt
	// must not be mirrored a second time
	env2, _ := testEnvironment(t, []string{"PRINT 1"})
	if err := env2.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	env2.Run()
	content := textPageString(env2.mem)
	assertContains(t, content, "]PRINT 1")
	/*
		Exactly three: the "APPLE ][" banner, the "]PRINT 1" line,
		and the prompt of the new wait for input. A duplicated
		prompt on the resume would add a fourth.
	*/
	if strings.Count(content, "]") != 3 {
		t.Errorf("duplicated prompt:\n%s", content)
	}
}

func TestTextSnapshot(t *testing.T) {
	env, _ := testEnvironment(t, []string{`PRINT "SNAPSHOT TEST"`})
	env.Run()
	img := screen.Snapshot(env.mem, screen.ScreenModeGreen)
	if img == nil {
		t.Fatal("no image generated")
	}
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		t.Errorf("empty image: %v", bounds)
	}
}
