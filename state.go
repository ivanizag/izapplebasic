package izapplebasic

import (
	"fmt"
	"io"
)

/*
The emulation state can be saved and loaded back, even on another
process. It has the CPU state, the RAM, the video mode and which
BASIC was running. The ROM itself is not included, it comes with the
binary, but the state has to say which one it was: a program and a
CPU of one BASIC restored on the ROM of the other would run as
nonsense instead of failing.

The state is saved while the machine waits on a GETLN call, that is
where the meta commands are processed. On load, the machine is left
waiting on that same GETLN: the run loop serves it with the next
input line and the restored program continues.

The core works on readers and writers, the files are on the hands
of the frontends.
*/

const stateMagic = "izapplebasic state v2\n"

// The v1 states have no language byte, they are all Applesoft. They
// are still read, there are saved states of the users out there.
const stateMagicV1 = "izapplebasic state v1\n"

const (
	stateFlagText  = uint8(1)
	stateFlagMixed = uint8(2)
	stateFlagPage2 = uint8(4)
	stateFlagHiRes = uint8(8)
)

func (env *Environment) SaveState(w io.Writer) error {
	if _, err := io.WriteString(w, stateMagic); err != nil {
		return err
	}
	if _, err := w.Write([]uint8{uint8(env.language)}); err != nil {
		return err
	}
	if err := env.cpu.Save(w); err != nil {
		return err
	}
	if _, err := w.Write(env.mem.data[:ioAreaStart]); err != nil {
		return err
	}
	var flags uint8
	if env.mem.textMode {
		flags |= stateFlagText
	}
	if env.mem.mixedMode {
		flags |= stateFlagMixed
	}
	if env.mem.page2 {
		flags |= stateFlagPage2
	}
	if env.mem.hiResMode {
		flags |= stateFlagHiRes
	}
	if _, err := w.Write([]uint8{flags, env.col}); err != nil {
		return err
	}
	return nil
}

func (env *Environment) LoadState(r io.Reader) error {
	magic := make([]uint8, len(stateMagic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return err
	}
	language := LanguageApplesoft
	switch string(magic) {
	case stateMagic:
		var b [1]uint8
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		if _, ok := romInfos[Language(b[0])]; !ok {
			return fmt.Errorf("the state uses an unknown BASIC")
		}
		language = Language(b[0])
	case stateMagicV1:
		// Before the language byte, always Applesoft
	default:
		return fmt.Errorf("not an izapplebasic state")
	}
	// The ROM has to be in place before the CPU and the RAM
	if language != env.language {
		if err := env.setLanguage(language); err != nil {
			return err
		}
	}
	if err := env.cpu.Load(r); err != nil {
		return err
	}
	if _, err := io.ReadFull(r, env.mem.data[:ioAreaStart]); err != nil {
		return err
	}
	var tail [2]uint8
	if _, err := io.ReadFull(r, tail[:]); err != nil {
		return err
	}
	flags := tail[0]
	env.mem.textMode = flags&stateFlagText != 0
	env.mem.mixedMode = flags&stateFlagMixed != 0
	env.mem.page2 = flags&stateFlagPage2 != 0
	env.mem.hiResMode = flags&stateFlagHiRes != 0
	env.col = tail[1]
	// The restored machine is past its boot, it waits for input
	env.pendingColdStart = false
	env.pendingIn = nil
	return nil
}
