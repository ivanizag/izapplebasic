package main

import _ "embed"

// The Apple II+ ROM, from 0xd000 to 0xffff, embedded in the binary
// and used unless another file is given with the -rom argument.
//
//go:embed original/Apple2_Plus.rom
var embeddedROM []uint8
