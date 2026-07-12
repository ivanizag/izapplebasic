package izapplebasic

import (
	"strings"
	"testing"
)

func TestMonitorTapeWriteRead(t *testing.T) {
	env, con := testEnvironment(t, []string{
		"CALL -151",
		"300:AB CD",
		"300.301W", // write one block from 0x300..0x301
		"310.311R", // read it back somewhere else
		"310.311",
	})
	// Rewind the tape before the read
	con.onReadLine = func(line string) {
		if line == "310.311R" {
			con.tapePos = 0
		}
	}
	env.Run()
	if len(con.tapeBlocks) != 1 || len(con.tapeBlocks[0]) != 2 {
		t.Fatalf("one block of 2 bytes expected, got %v", con.tapeBlocks)
	}
	assertContains(t, con.output, "0310- AB CD")
	if strings.Contains(con.output, "ERR") {
		t.Errorf("no error expected:\n%s", con.output)
	}
}

func TestApplesoftSaveLoad(t *testing.T) {
	env, con := testEnvironment(t, []string{
		`10 PRINT "FROM THE TAPE"`,
		"SAVE",
		"NEW",
		"LOAD",
		"RUN",
	})
	// Rewind the tape before the LOAD
	con.onReadLine = func(line string) {
		if line == "LOAD" {
			con.tapePos = 0
		}
	}
	env.Run()
	// Applesoft SAVE writes a length header block and the program
	if len(con.tapeBlocks) != 2 {
		t.Fatalf("two blocks expected, got %v", len(con.tapeBlocks))
	}
	assertContains(t, con.output, "FROM THE TAPE")
}

func TestTapeReadEndOfTape(t *testing.T) {
	out := runBasic(t, []string{
		"CALL -151",
		"300.30FR",
	})
	assertContains(t, out, "ERR")
}

func TestTapeReadSizeMismatch(t *testing.T) {
	env, con := testEnvironment(t, []string{
		"CALL -151",
		"300:11 22",
		"300.301W",
		"310.313R", // asks for 4 bytes, the block has 2
		"310.311",
	})
	con.onReadLine = func(line string) {
		if line == "310.313R" {
			con.tapePos = 0
		}
	}
	env.Run()
	// The available bytes are copied and the error is reported
	assertContains(t, con.output, "ERR")
	assertContains(t, con.output, "0310- 11 22")
}
