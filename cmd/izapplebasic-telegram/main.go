package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	token := flag.String(
		"token",
		"",
		"telegram bot token (default: the TELEGRAM_TOKEN environment variable)")
	dataDir := flag.String(
		"data",
		"telegram-data",
		"directory for the users state and logs")

	flag.Parse()

	if *token == "" {
		*token = os.Getenv("TELEGRAM_TOKEN")
	}

	err := runTelegram(*token, *dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
