package main

import (
	"strings"
	"testing"
)

func testCout(env *environment, ch uint8) {
	_, x, y, p := env.cpu.GetAXYP()
	env.cpu.SetAXYP(ch|0x80, x, y, p)
	execCOUT1(env)
}

func TestCOUT1Chars(t *testing.T) {
	env, con := testEnvironment(t, nil)
	testCout(env, 'H')
	testCout(env, 'I')
	if con.output != "HI" {
		t.Errorf("got %q", con.output)
	}
	if env.mem.Peek(zpCH) != 2 {
		t.Error("CH must track the column")
	}
}

func TestCOUT1CarriageReturn(t *testing.T) {
	env, con := testEnvironment(t, nil)
	testCout(env, 'A')
	testCout(env, '\r')
	if con.output != "A\n" {
		t.Errorf("got %q", con.output)
	}
	if env.mem.Peek(zpCH) != 0 {
		t.Error("CH must be zero after a CR")
	}
}

func TestCOUT1ColumnJump(t *testing.T) {
	// Applesoft moves the cursor for the print zones and HTAB by
	// changing CH directly, the output must be padded with spaces.
	env, con := testEnvironment(t, nil)
	testCout(env, 'A')
	env.mem.Poke(zpCH, 5)
	testCout(env, 'B')
	if con.output != "A    B" {
		t.Errorf("got %q", con.output)
	}
	if env.mem.Peek(zpCH) != 6 {
		t.Errorf("CH must be 6, got %v", env.mem.Peek(zpCH))
	}
}

func TestCOUT1BackwardsJumpIgnored(t *testing.T) {
	// On a stream we cannot move the cursor back, the char is
	// appended at the current position.
	env, con := testEnvironment(t, nil)
	testCout(env, 'A')
	testCout(env, 'B')
	env.mem.Poke(zpCH, 0)
	testCout(env, 'C')
	if con.output != "ABC" {
		t.Errorf("got %q", con.output)
	}
}

func TestHOMEIgnoredByDefault(t *testing.T) {
	out := runBasic(t, []string{"HOME"})
	if strings.Contains(out, "\f") {
		t.Error("HOME must not clear the screen by default")
	}
}

func TestHOMEClearsWhenEnabled(t *testing.T) {
	env, con := testEnvironment(t, []string{"HOME"})
	env.clearScreen = true
	run(env)
	assertContains(t, con.output, "\f")
}
