package izapplebasic

import (
	"bytes"
	"strings"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	env1, _ := testEnvironment(t, nil)
	env1.mem.Poke(0x1234, 0xda)
	env1.mem.Peek(ioGraphics)
	env1.mem.Peek(ioHiRes)
	env1.cpu.SetPC(0xabcd)
	env1.col = 7
	var buf bytes.Buffer
	if err := env1.SaveState(&buf); err != nil {
		t.Fatal(err)
	}

	env2, _ := testEnvironment(t, nil)
	if err := env2.LoadState(&buf); err != nil {
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

func TestStateBadData(t *testing.T) {
	env, _ := testEnvironment(t, nil)
	err := env.LoadState(strings.NewReader("this is not a state, not at all, no"))
	if err == nil || !strings.Contains(err.Error(), "not an izapplebasic state") {
		t.Errorf("bad data must be rejected, got: %v", err)
	}
}

func TestStateAcrossSessions(t *testing.T) {
	// First session: define a program and a variable, save
	env1, con1 := testEnvironment(t, []string{
		`10 PRINT "PERSISTED"`,
		"X=21",
	})
	env1.Run()
	var buf bytes.Buffer
	if err := env1.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	assertContains(t, con1.output, "]X=21")

	// Second session: load and continue where it was left
	env2, con2 := testEnvironment(t, []string{
		"PRINT X*2",
		"RUN",
	})
	if err := env2.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	env2.Run()
	assertContains(t, con2.output, "42")
	assertContains(t, con2.output, "PERSISTED")
}
