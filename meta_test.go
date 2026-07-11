package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetaScreenshotGr(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "test.png")
	out := runBasic(t, []string{
		"GR",
		"COLOR=9",
		"PLOT 5,5",
		"/screenshot " + filename,
	})
	assertContains(t, out, "screenshot saved to "+filename)

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

func TestMetaScreenshotHgr(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "test.png")
	out := runBasic(t, []string{
		"HGR",
		"HPLOT 0,0 TO 100,100",
		"/screenshot " + filename,
	})
	assertContains(t, out, "screenshot saved to "+filename)
	if _, err := os.Stat(filename); err != nil {
		t.Fatal(err)
	}
}

func TestMetaCommandNotSeenByBasic(t *testing.T) {
	out := runBasic(t, []string{
		"/nosuchcommand",
		"PRINT 7*6",
	})
	assertContains(t, out, "unknown meta command /nosuchcommand")
	assertContains(t, out, "42")
	if strings.Contains(out, "SYNTAX") {
		t.Errorf("the meta command must not reach Applesoft:\n%s", out)
	}
}

func TestMetaHelp(t *testing.T) {
	out := runBasic(t, []string{"/help"})
	assertContains(t, out, "/screenshot")
}

func TestMetaQuit(t *testing.T) {
	out := runBasic(t, []string{
		"/quit",
		"PRINT 123",
	})
	if strings.Contains(out, "123") {
		t.Errorf("no input must be processed after /quit:\n%s", out)
	}
}

func TestLowercaseInput(t *testing.T) {
	out := runBasic(t, []string{`print 2+2`})
	assertContains(t, out, "4")
}
