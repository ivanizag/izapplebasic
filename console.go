package izapplebasic

// Console is the interface implemented by the frontends. The
// emulation core uses it for all the input and output intercepted
// from the monitor ROM calls. The core does not touch the host
// filesystem: anything involving files lives on the frontends.
type Console interface {
	// ReadLine returns a full line of input, without the final CR.
	// The second value is true when there is no more input (EOF).
	ReadLine(prompt string) (string, bool)

	// ReadChar returns a single character of input.
	// The second value is true when there is no more input (EOF).
	ReadChar() (uint8, bool)

	// Write sends text to the user.
	Write(string)

	// Clear clears the screen, if the frontend supports it.
	Clear()

	// MetaCommand gives the frontend the chance to process an input
	// line as one of its meta commands, returning true when the
	// line was consumed and must not reach the emulated machine.
	MetaCommand(line string) bool
}
