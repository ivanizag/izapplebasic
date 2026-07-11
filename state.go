package izapplebasic

import (
	"fmt"
	"io"
)

/*
The emulation state can be saved and loaded back, even on another
process. It has the CPU state, the RAM and the video mode. The ROM
is not included, it comes with the binary.

The state is saved while the machine waits on a GETLN call, that is
where the meta commands are processed. On load, the machine is left
waiting on that same GETLN: the run loop serves it with the next
input line and the restored program continues.

The core works on readers and writers, the files are on the hands
of the frontends.
*/

const stateMagic = "izapplebasic state v1\n"

const (
	stateFlagText        = uint8(1)
	stateFlagMixed       = uint8(2)
	stateFlagPage2       = uint8(4)
	stateFlagHiRes       = uint8(8)
	stateFlagPromptShown = uint8(16)
)

func (env *Environment) SaveState(w io.Writer) error {
	if _, err := io.WriteString(w, stateMagic); err != nil {
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
	if env.promptShown {
		flags |= stateFlagPromptShown
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
	if string(magic) != stateMagic {
		return fmt.Errorf("not an izapplebasic state")
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
	env.promptShown = flags&stateFlagPromptShown != 0
	env.col = tail[1]
	return nil
}
