package izapplebasic

import (
	"strings"
	"testing"
)

func TestIntegerBoot(t *testing.T) {
	out := runInteger(t, []string{"PRINT 2+2"})
	assertContains(t, out, ">")
	assertContains(t, out, "4")
	if strings.Contains(out, "*") {
		t.Errorf("the boot stopped on the monitor prompt:\n%s", out)
	}
}

func TestIntegerPrint(t *testing.T) {
	out := runInteger(t, []string{`PRINT "HELLO"`})
	assertContains(t, out, "HELLO")
}

func TestIntegerRunAndList(t *testing.T) {
	out := runInteger(t, []string{
		"10 FOR I=1 TO 3",
		"20 PRINT I",
		"30 NEXT I",
		"RUN",
		"LIST",
	})
	assertContains(t, out, "1\n2\n3")
	assertContains(t, out, "10 FOR I=1 TO 3")
	assertContains(t, out, "30 NEXT I")
}

// The Integer BASIC errors do not look like the Applesoft ones
func TestIntegerSyntaxError(t *testing.T) {
	out := runInteger(t, []string{"PRINT )("})
	assertContains(t, out, "SYNTAX ERR")
}

/*
Integer BASIC reads its direct mode key by key instead of calling
GETLN, the meta commands have to be served on that path too: it is
the only prompt the user ever sees on Integer BASIC.
*/
func TestIntegerMetaCommand(t *testing.T) {
	env, con := testEnvironmentWithLanguage(t, LanguageInteger, []string{
		"/meta",
		"PRINT 42",
	})
	metaCalls := 0
	con.metaFn = func(line string) bool {
		if strings.HasPrefix(line, "/") {
			metaCalls++
			return true
		}
		return false
	}
	env.Run()
	if metaCalls != 1 {
		t.Errorf("one meta command expected, got %v", metaCalls)
	}
	assertContains(t, con.output, "42")
	if strings.Contains(con.output, "SYNTAX") {
		t.Errorf("the meta command must not reach Integer BASIC:\n%s", con.output)
	}
}

// A reset from a meta command abandons the keystroke wait it was
// serving, the input after it belongs to the machine that booted
func TestIntegerResetFromMetaCommand(t *testing.T) {
	env, con := testEnvironmentWithLanguage(t, LanguageInteger, []string{
		"X=42",
		"/reset",
		"PRINT X",
	})
	con.metaFn = func(line string) bool {
		if line == "/reset" {
			env.Reset()
			return true
		}
		return false
	}
	env.Run()
	// The whole line reaches the rebooted machine, where X is 0: a
	// key served to the abandoned wait would have been lost here
	assertContains(t, con.output, ">PRINT X\n0")
}

// CALL -151 enters the monitor at MONZ, past the address used to
// enter the BASIC on the boot: it must not cold start again.
func TestIntegerMonitor(t *testing.T) {
	out := runInteger(t, []string{"CALL -151", "E000.E002"})
	assertContains(t, out, "*")
	assertContains(t, out, "E000-")
}
