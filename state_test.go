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

// A state of one BASIC restored on an environment built for the
// other one has to bring its ROM along, the Telegram frontend
// always builds the machine on Applesoft and loads the state after.
func TestStateCarriesTheLanguage(t *testing.T) {
	env1, con1 := testEnvironmentWithLanguage(t, LanguageInteger, []string{
		`10 PRINT "INTEGER"`,
	})
	env1.Run()
	assertContains(t, con1.output, ">10 PRINT")
	var buf bytes.Buffer
	if err := env1.SaveState(&buf); err != nil {
		t.Fatal(err)
	}

	env2, con2 := testEnvironment(t, []string{"RUN"})
	if err := env2.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	if env2.Language() != LanguageInteger {
		t.Error("the language must be restored")
	}
	if env2.mem.Peek(0xfffc) != 0x59 || env2.mem.Peek(0xfffd) != 0xff {
		t.Error("the Integer BASIC ROM must be in place")
	}
	env2.Run()
	assertContains(t, con2.output, "INTEGER")
}

// The states saved before the language byte are all Applesoft
func TestStateV1LoadsAsApplesoft(t *testing.T) {
	env1, _ := testEnvironment(t, nil)
	env1.mem.Poke(0x1234, 0xda)
	var buf bytes.Buffer
	if err := env1.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	// Downgrade the saved state to the v1 format: the magic of the
	// older version and no language byte
	v1 := append([]uint8(stateMagicV1), buf.Bytes()[len(stateMagic)+1:]...)

	env2, _ := testEnvironmentWithLanguage(t, LanguageInteger, nil)
	if err := env2.LoadState(bytes.NewReader(v1)); err != nil {
		t.Fatal(err)
	}
	if env2.Language() != LanguageApplesoft {
		t.Error("a v1 state must load as Applesoft")
	}
	if env2.mem.Peek(0x1234) != 0xda {
		t.Error("the RAM of a v1 state must be restored")
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
