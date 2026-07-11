package main

import (
	"strings"
	"testing"
	"time"
)

func TestBoot(t *testing.T) {
	out := runBasic(t, nil)
	assertContains(t, out, "]")
}

func TestPrintExpression(t *testing.T) {
	out := runBasic(t, []string{
		`PRINT 2+2`,
	})
	assertContains(t, out, "4")
}

func TestPrintString(t *testing.T) {
	out := runBasic(t, []string{
		`PRINT "HELLO, WORLD!"`,
	})
	assertContains(t, out, "HELLO, WORLD!")
}

func TestStringFunctions(t *testing.T) {
	out := runBasic(t, []string{
		`PRINT LEFT$("APPLESOFT", 5) + " " + STR$(LEN("II+"))`,
	})
	assertContains(t, out, "APPLE 3")
}

func TestRunProgram(t *testing.T) {
	out := runBasic(t, []string{
		"10 FOR I=1 TO 5",
		"20 PRINT I*I",
		"30 NEXT",
		"RUN",
	})
	for _, want := range []string{"1", "4", "9", "16", "25"} {
		assertContains(t, out, want+"\n")
	}
}

func TestList(t *testing.T) {
	out := runBasic(t, []string{
		`10 PRINT "A"`,
		"LIST",
	})
	// LIST reformats the line with extra spaces
	assertContains(t, out, `10  PRINT "A"`)
}

func TestPrintZones(t *testing.T) {
	out := runBasic(t, []string{
		`PRINT "A","B","C"`,
	})
	// The comma zones are 16 chars wide
	assertContains(t, out, "A"+strings.Repeat(" ", 15)+"B"+strings.Repeat(" ", 15)+"C")
}

func TestTab(t *testing.T) {
	out := runBasic(t, []string{
		`PRINT TAB(10);"X"`,
	})
	assertContains(t, out, strings.Repeat(" ", 9)+"X")
}

func TestHtab(t *testing.T) {
	out := runBasic(t, []string{
		`HTAB 21: PRINT "X"`,
	})
	assertContains(t, out, strings.Repeat(" ", 20)+"X")
}

func TestInput(t *testing.T) {
	out := runBasic(t, []string{
		"10 INPUT A",
		"20 PRINT A*2",
		"RUN",
		"21",
		"PRINT A+1",
	})
	assertContains(t, out, "?") // the INPUT prompt
	assertContains(t, out, "42")
	assertContains(t, out, "22")
}

func TestInputString(t *testing.T) {
	out := runBasic(t, []string{
		`10 INPUT "NAME? "; N$`,
		`20 PRINT "HELLO " + N$`,
		"RUN",
		"IVAN",
	})
	assertContains(t, out, "NAME? ")
	assertContains(t, out, "HELLO IVAN")
}

func TestGet(t *testing.T) {
	out := runBasic(t, []string{
		`10 GET K$`,
		`20 PRINT "GOT ";K$`,
		"RUN",
		"Q",
	})
	assertContains(t, out, "GOT Q")
}

func TestSyntaxError(t *testing.T) {
	out := runBasic(t, []string{
		"THIS IS NOT BASIC",
	})
	assertContains(t, out, "?SYNTAX ERROR")
}

func TestDivisionByZeroError(t *testing.T) {
	out := runBasic(t, []string{
		"PRINT 1/0",
	})
	assertContains(t, out, "?DIVISION BY ZERO ERROR")
}

func TestGosub(t *testing.T) {
	out := runBasic(t, []string{
		"10 GOSUB 100",
		`20 PRINT "AFTER"`,
		"30 END",
		`100 PRINT "SUB"`,
		"110 RETURN",
		"RUN",
	})
	assertContains(t, out, "SUB\nAFTER")
}

func TestFloatingPoint(t *testing.T) {
	out := runBasic(t, []string{
		"PRINT SQR(2)",
	})
	assertContains(t, out, "1.41421356")
}

func TestPeekPoke(t *testing.T) {
	out := runBasic(t, []string{
		"POKE 768,123",
		"PRINT PEEK(768)",
	})
	assertContains(t, out, "123")
}

func TestBreakInfiniteLoop(t *testing.T) {
	env, con := testEnvironment(t, []string{
		"10 GOTO 10",
		"RUN",
	})
	// Simulate a control-C keypress while the program runs
	go func() {
		time.Sleep(10 * time.Millisecond)
		env.mem.breakPending.Store(true)
	}()
	run(env)
	if env.cpu.GetCycles() >= testCyclesLimit {
		t.Log(con.output)
		t.Fatal("the program was not interrupted")
	}
	// Applesoft embeds a BEL in the error messages
	assertContains(t, con.output, "BREAK\a IN 10")
}

func TestMonitorFromBasic(t *testing.T) {
	// CALL -151 enters the monitor, which runs on the same
	// intercepted GETLN and COUT routines
	out := runBasic(t, []string{
		"CALL -151",
		"300:AB",
		"300",
		"E000",
	})
	assertContains(t, out, "*")        // the monitor prompt
	assertContains(t, out, "0300- AB") // memory read back
	assertContains(t, out, "E000- 4C") // the BASIC entry JMP
}
