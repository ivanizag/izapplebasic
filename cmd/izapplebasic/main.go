package main

import (
	"flag"
	"fmt"
	"os"

	iz "github.com/ivanizag/izapplebasic"
)

func main() {
	languageName := flag.String(
		"language",
		"applesoft",
		"BASIC to boot: applesoft or integer")
	romFilename := flag.String(
		"rom",
		"",
		"filename of an Apple II+ ROM, 12 KB from 0xd000 or 8 KB from 0xe000 (default: embedded ROM)")
	traceCPU := flag.Bool(
		"c",
		false,
		"dump to the console the CPU execution operations")
	traceMonitor := flag.Bool(
		"m",
		false,
		"dump to the console the intercepted monitor calls excluding char output")
	traceMonitorFull := flag.Bool(
		"M",
		false,
		"dump to the console the intercepted monitor calls including char output")
	traceIO := flag.Bool(
		"s",
		false,
		"dump to the console the accesses to the softswitches at 0xc0xx")
	clearScreen := flag.Bool(
		"home",
		false,
		"clear the host screen on HOME (default: ignored)")
	noUppercase := flag.Bool(
		"l",
		false,
		"do not convert the input to uppercase")
	rawline := flag.Bool(
		"r",
		false,
		"disable readline like input with history")
	loadFilename := flag.String(
		"load",
		"",
		"load the emulation state from a file on startup")
	tapeName := flag.String(
		"tape",
		"default",
		"name of the cassette tape inserted on startup")

	flag.Parse()

	if !validTapeName.MatchString(*tapeName) {
		fmt.Fprintln(os.Stderr, "Error: invalid tape name, use up to 30 letters, digits, - or _")
		os.Exit(1)
	}

	language, ok := iz.ParseLanguage(*languageName)
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: unknown language, use applesoft or integer")
		os.Exit(1)
	}

	var env *iz.Environment
	var err error
	if *romFilename != "" {
		// A ROM given as a file is booted through the reset vector,
		// as the Apple II+ one
		var rom []uint8
		rom, err = os.ReadFile(*romFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		env, err = iz.NewEnvironment(rom)
	} else {
		env, err = iz.NewEnvironmentWithLanguage(language)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	env.Uppercase = !*noUppercase
	env.TraceMonitor = *traceMonitor || *traceMonitorFull
	env.TraceMonitorIO = *traceMonitorFull
	env.SetTraceIO(*traceIO)
	env.SetTraceCPU(*traceCPU)

	esc := newEscaper(env)
	tape := newTapeDrive(".")
	tape.name = *tapeName
	tape.trace = *traceMonitor || *traceMonitorFull
	var con console
	if *rawline || !stdinIsTerminal() {
		con = newConsoleStdio(env, tape, *clearScreen)
	} else {
		con = newConsoleLiner(env, tape, esc, *clearScreen)
	}
	env.SetConsole(con)
	esc.closeFn = con.close
	defer con.close()

	if *loadFilename != "" {
		err := loadStateFile(env, *loadFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	handleControlC(esc)

	fmt.Printf("izapplebasic - %s on modern hardware, https://github.com/ivanizag/izapplebasic\n",
		env.Language().Name())
	fmt.Println("(type /help for the meta commands, press control-c twice to exit)")

	env.Run()
	fmt.Println()
}

// console extends the core Console interface with the local cleanup.
type console interface {
	iz.Console
	close()
}

// stdinIsTerminal returns false when the input is piped or
// redirected, the line editing is then disabled.
func stdinIsTerminal() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}
