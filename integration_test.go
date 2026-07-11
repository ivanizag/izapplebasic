package main

import (
	"strings"
	"testing"
)

// testCyclesLimit stops a test if a BASIC program loops forever.
const testCyclesLimit = 200_000_000

// testEnvironment builds an environment ready to run with a mock
// console fed with the given input lines.
func testEnvironment(t *testing.T, linesIn []string) (*environment, *consoleMock) {
	t.Helper()
	con := newConsoleMock(linesIn)
	env, err := newEnvironment(embeddedROM)
	if err != nil {
		t.Fatal(err)
	}
	env.con = con
	env.maxCycles = testCyclesLimit
	return env, con
}

// runBasic boots the machine, types the given lines on the Applesoft
// prompt and returns the session transcript.
func runBasic(t *testing.T, linesIn []string) string {
	t.Helper()
	env, con := testEnvironment(t, linesIn)
	run(env)
	if env.cpu.GetCycles() >= testCyclesLimit {
		t.Log(con.output)
		t.Fatal("the program did not finish within the cycles limit")
	}
	return con.output
}

func assertContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Errorf("output does not contain %q:\n%s", want, output)
	}
}
