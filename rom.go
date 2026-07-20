package izapplebasic

import (
	_ "embed"
	"strings"
)

// The Apple II+ ROM, from 0xd000 to 0xffff, with Applesoft and the
// autostart monitor. Used unless another one is chosen with the
// language, or given as a file with the -rom argument.
//
//go:embed original/Apple2_Plus.rom
var applesoftROM []uint8

// The Apple II ROM, from 0xe000 to 0xffff, with Integer BASIC and
// the original monitor. There is nothing on 0xd000 to 0xdfff, that
// was the socket for the Programmer's Aid.
//
//go:embed original/341-000x_integer.rom
var integerROM []uint8

// The character generator ROM with the character bitmaps, used to
// render the text screen on the snapshots. Same one used in
// izapple2.
//
//go:embed original/Apple2rev7CharGen.rom
var embeddedCharGen []uint8

// Language is the BASIC in ROM.
type Language uint8

const (
	LanguageApplesoft Language = 0
	LanguageInteger   Language = 1
)

/*
The two ROMs boot differently. The autostart monitor of the Apple
II+ scans the slots for a bootable card on reset, finds none as they
all read zeros, and falls back to Applesoft on its own. The original
monitor of the Apple II has no autostart: its reset vector leaves
the machine on the "*" monitor prompt, Integer BASIC has to be
entered explicitly on its cold start address.
*/
type romInfo struct {
	name string
	data []uint8

	// coldStart is where to enter the BASIC after the reset, or 0
	// when the reset vector gets there by itself.
	coldStart uint16
}

var romInfos = map[Language]romInfo{
	LanguageApplesoft: {name: "Applesoft BASIC", data: applesoftROM, coldStart: 0},
	LanguageInteger:   {name: "Integer BASIC", data: integerROM, coldStart: 0xe000},
}

func (l Language) info() romInfo {
	return romInfos[l]
}

// Name returns the name of the BASIC, to show to the user.
func (l Language) Name() string {
	return l.info().name
}

// ParseLanguage resolves the name of a BASIC as the frontends take
// it from the user, on the command line or on a meta command.
func ParseLanguage(name string) (Language, bool) {
	switch strings.ToLower(name) {
	case "applesoft":
		return LanguageApplesoft, true
	case "integer":
		return LanguageInteger, true
	}
	return 0, false
}
