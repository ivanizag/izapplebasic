package izapplebasic

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

/*
The BASIC program in memory can be read back as text, the way LIST
prints it, to be written to a .BAS file by the frontends.

Applesoft keeps the program as a chain of lines starting at TXTTAB:
two bytes with the address of the next line, zero on the last one,
two bytes with the line number, and the tokenized body terminated by
a zero. A byte with the high bit set is a token, anything below is a
literal character, so the quoted strings and the REM text need no
special handling.

The token names are not hardcoded here, they are read from the token
name table of the ROM: the keywords in ASCII with the high bit set on
the last character of each one. A ROM given with -rom brings its own.
*/

// ErrExportNotSupported is returned for a BASIC whose stored program
// format is not decoded yet.
var ErrExportNotSupported = errors.New("the program can only be exported from Applesoft BASIC")

const (
	zpTXTTAB = uint16(0x67) // Start of the Applesoft program
	zpVARTAB = uint16(0x69) // Start of the variables, past the program

	// TOKEN_NAME_TABLE of the Applesoft ROM, see Applesoft.dis65
	addrTokenNames = uint16(0xd0d0)
	tokenNamesEnd  = uint16(0xd400) // Bound for a ROM with no table
	firstToken     = uint8(0x80)

	// A line takes at least five bytes and the program lives on the
	// 48 KB of RAM, this only stops a chain that never ends
	maxProgramLines = 16384
)

// ExportProgram returns the BASIC program in memory as text, laid
// out as LIST prints it.
func (env *Environment) ExportProgram() (string, error) {
	if env.language != LanguageApplesoft {
		return "", ErrExportNotSupported
	}
	return env.exportApplesoft()
}

func (env *Environment) peekWord(address uint16) uint16 {
	return uint16(env.mem.Peek(address)) + uint16(env.mem.Peek(address+1))<<8
}

// applesoftTokenNames reads the keyword of each token from the ROM,
// indexed by token minus 0x80.
func (env *Environment) applesoftTokenNames() []string {
	var names []string
	var name strings.Builder
	for address := addrTokenNames; address < tokenNamesEnd; address++ {
		b := env.mem.Peek(address)
		if b == 0 {
			break
		}
		name.WriteByte(b & 0x7f)
		if b&0x80 != 0 {
			// The high bit closes the keyword
			names = append(names, name.String())
			name.Reset()
		}
	}
	return names
}

/*
exportApplesoft walks the chain of lines. LIST prints the line
number and a space, then every token surrounded by spaces and every
other byte as it is: that is why "20 A=1" comes back as "20 A = 1".
The trailing space of a line ending on a token is trimmed, it would
only be whitespace on the file.
*/
func (env *Environment) exportApplesoft() (string, error) {
	names := env.applesoftTokenNames()
	if len(names) == 0 {
		return "", errors.New("the ROM has no Applesoft token name table")
	}
	vartab := env.peekWord(zpVARTAB)

	var program strings.Builder
	address := env.peekWord(zpTXTTAB)
	for count := 0; ; count++ {
		next := env.peekWord(address)
		if next == 0 {
			// The end of the program
			return program.String(), nil
		}
		if next <= address || next >= vartab || count >= maxProgramLines {
			return "", fmt.Errorf("the memory does not hold a valid Applesoft program")
		}

		var line strings.Builder
		line.WriteString(strconv.Itoa(int(env.peekWord(address + 2))))
		line.WriteByte(' ')
		// The body ends on a zero, the next line bounds a corrupt one
		for p := address + 4; p < next; p++ {
			b := env.mem.Peek(p)
			if b == 0 {
				break
			}
			if b < firstToken {
				line.WriteByte(b)
				continue
			}
			if token := int(b - firstToken); token < len(names) {
				line.WriteString(" " + names[token] + " ")
			} else {
				// No keyword for it, keep it visible instead of
				// printing the garbage past the table as LIST does
				line.WriteString(fmt.Sprintf(" $%02X ", b))
			}
		}
		program.WriteString(strings.TrimRight(line.String(), " "))
		program.WriteByte('\n')

		address = next
	}
}
