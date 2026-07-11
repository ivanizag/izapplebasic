package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

/*
The emulation state can be saved to a file and loaded back, even on
another session. It has the CPU state, the RAM and the video mode.
The ROM is not included, it comes with the binary.

The state is saved while the machine waits on a GETLN call, that is
where the meta commands are processed. On load, the machine is left
waiting on that same GETLN: the run loop serves it with the next
input line and the restored program continues.
*/

const stateMagic = "izapplebasic state v1\n"

const (
	stateFlagText  = uint8(1)
	stateFlagMixed = uint8(2)
	stateFlagPage2 = uint8(4)
	stateFlagHiRes = uint8(8)
)

func (env *environment) saveState(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	if _, err := w.WriteString(stateMagic); err != nil {
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
	if err := w.WriteByte(flags); err != nil {
		return err
	}
	if err := w.WriteByte(env.col); err != nil {
		return err
	}
	return w.Flush()
}

func (env *environment) loadState(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)

	magic := make([]uint8, len(stateMagic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return err
	}
	if string(magic) != stateMagic {
		return fmt.Errorf("%s is not an izapplebasic state file", filename)
	}
	if err := env.cpu.Load(r); err != nil {
		return err
	}
	if _, err := io.ReadFull(r, env.mem.data[:ioAreaStart]); err != nil {
		return err
	}
	flags, err := r.ReadByte()
	if err != nil {
		return err
	}
	env.mem.textMode = flags&stateFlagText != 0
	env.mem.mixedMode = flags&stateFlagMixed != 0
	env.mem.page2 = flags&stateFlagPage2 != 0
	env.mem.hiResMode = flags&stateFlagHiRes != 0
	env.col, err = r.ReadByte()
	if err != nil {
		return err
	}
	return nil
}
