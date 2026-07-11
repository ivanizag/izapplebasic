package izapplebasic

import (
	"strings"
	"testing"
)

func testCout(env *Environment, ch uint8) {
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

func TestHOMEClearsTheConsole(t *testing.T) {
	// The console decides what its Clear does, the frontends may
	// ignore it. The mock records a form feed.
	out := runBasic(t, []string{`PRINT "X"`, "HOME"})
	assertContains(t, out, "\f")
}

func TestMetaCommandNotSeenByBasic(t *testing.T) {
	env, con := testEnvironment(t, []string{
		"/meta",
		"PRINT 7*6",
	})
	metaCalls := 0
	con.metaFn = func(line string) bool {
		metaCalls++
		return true
	}
	env.Run()
	if metaCalls != 1 {
		t.Errorf("one meta command expected, got %v", metaCalls)
	}
	assertContains(t, con.output, "42")
	if strings.Contains(con.output, "SYNTAX") {
		t.Errorf("the meta command must not reach Applesoft:\n%s", con.output)
	}
}

func TestMetaCommandCanStop(t *testing.T) {
	env, con := testEnvironment(t, []string{
		"/quit",
		"PRINT 123",
	})
	con.metaFn = func(line string) bool {
		env.Stop()
		return true
	}
	env.Run()
	if strings.Contains(con.output, "123") {
		t.Errorf("no input must be processed after the stop:\n%s", con.output)
	}
}

func TestResetFromMetaCommand(t *testing.T) {
	env, con := testEnvironment(t, []string{
		"X=42",
		"/reset",
		"PRINT X",
	})
	con.metaFn = func(line string) bool {
		env.Reset()
		return true
	}
	env.Run()
	// The machine rebooted, X is gone, and the line after the
	// reset is served by the new boot prompt
	assertContains(t, con.output, "\n0\n")
}

func TestLowercaseInput(t *testing.T) {
	out := runBasic(t, []string{`print 2+2`})
	assertContains(t, out, "4")
}
