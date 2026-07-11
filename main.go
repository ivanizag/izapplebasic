package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	romFilename := flag.String(
		"rom",
		"",
		"filename of the Apple II+ ROM, 12 KB from 0xd000 to 0xffff (default: embedded ROM)")
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

	flag.Parse()

	rom := embeddedROM
	if *romFilename != "" {
		var err error
		rom, err = os.ReadFile(*romFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	env, err := newEnvironment(rom)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if *rawline || !stdinIsTerminal() {
		env.con = newConsoleStdio(!*noUppercase)
	} else {
		env.con = newConsoleLiner(env, !*noUppercase)
	}
	defer env.con.close()
	env.apiLog = *traceMonitor || *traceMonitorFull
	env.apiLogIO = *traceMonitorFull
	env.clearScreen = *clearScreen
	env.mem.traceIO = *traceIO
	env.cpu.SetTrace(*traceCPU)

	handleControlC(env)

	fmt.Println("izapplebasic - Applesoft BASIC on modern hardware, https://github.com/ivanizag/izapplebasic")
	fmt.Println("(press control-c twice to exit)")

	run(env)
	fmt.Println()
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

// handleControlC breaks the running BASIC program on control-C
// instead of killing the process. Two in fast succession quit.
func handleControlC(env *environment) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for range c {
			env.escape()
		}
	}()
}
