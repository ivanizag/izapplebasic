package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(filename string, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func TestStateRoundTrip(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "test.state")

	env1, _ := testEnvironment(t, nil)
	env1.mem.Poke(0x1234, 0xda)
	env1.mem.Peek(ioGraphics)
	env1.mem.Peek(ioHiRes)
	env1.cpu.SetPC(0xabcd)
	env1.col = 7
	if err := env1.saveState(filename); err != nil {
		t.Fatal(err)
	}

	env2, _ := testEnvironment(t, nil)
	if err := env2.loadState(filename); err != nil {
		t.Fatal(err)
	}
	if env2.mem.Peek(0x1234) != 0xda {
		t.Error("the RAM must be restored")
	}
	if !env2.mem.hiResMode || env2.mem.textMode {
		t.Error("the video mode must be restored")
	}
	if pc, _ := env2.cpu.GetPCAndSP(); pc != 0xabcd {
		t.Errorf("the CPU state must be restored, PC is %04x", pc)
	}
	if env2.col != 7 {
		t.Error("the column must be restored")
	}
}

func TestStateBadFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "bad.state")
	env, _ := testEnvironment(t, nil)

	if err := env.loadState(filename); err == nil {
		t.Error("a missing file must be an error")
	}

	if err := writeFile(filename, "this is not a state file, not at all, no"); err != nil {
		t.Fatal(err)
	}
	if err := env.loadState(filename); err == nil ||
		!strings.Contains(err.Error(), "not an izapplebasic state file") {
		t.Errorf("a bad file must be rejected, got: %v", err)
	}
}

func TestStateAcrossSessions(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "session.state")

	// First session: define a program and a variable, save
	out := runBasic(t, []string{
		`10 PRINT "PERSISTED"`,
		"X=21",
		"/save " + filename,
	})
	assertContains(t, out, "state saved to "+filename)

	// Second session: load and continue where it was left
	out = runBasic(t, []string{
		"/load " + filename,
		"PRINT X*2",
		"RUN",
	})
	assertContains(t, out, "state loaded from "+filename)
	assertContains(t, out, "42")
	assertContains(t, out, "PERSISTED")
}
