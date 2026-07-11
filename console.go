package main

// console is the interface implemented by the frontends. The
// emulation core uses it for all the input and output intercepted
// from the monitor ROM calls.
type console interface {
	// readLine returns a full line of input, without the final CR.
	// The second value is true when there is no more input (EOF).
	readLine(prompt string) (string, bool)

	// readChar returns a single character of input.
	// The second value is true when there is no more input (EOF).
	readChar() (uint8, bool)

	// write sends text to the user.
	write(string)

	// clear clears the screen, if the frontend supports it.
	clear()

	// close releases the frontend resources.
	close()
}
