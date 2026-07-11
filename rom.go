package izapplebasic

import _ "embed"

// The Apple II+ ROM, from 0xd000 to 0xffff, embedded in the binary
// and used unless another file is given with the -rom argument.
//
//go:embed original/Apple2_Plus.rom
var embeddedROM []uint8

// The character generator ROM with the character bitmaps, used to
// render the text screen on the snapshots. Same one used in
// izapple2.
//
//go:embed original/Apple2rev7CharGen.rom
var embeddedCharGen []uint8
