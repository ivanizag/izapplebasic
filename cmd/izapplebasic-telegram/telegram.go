package main

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	iz "github.com/ivanizag/izapplebasic"
)

/*
Telegram frontend. It connects to Telegram with long polling, no
webhook or open port needed.

The emulation is stateless between messages: on each message the
state of the user is loaded from a file, the lines of the message
are executed, and the state is saved back. Every user has a
directory with the state and a log of the interaction.
*/

const (
	// Cycles allowed per message before injecting a control-C to
	// break a long running program: 100 emulated seconds, under a
	// second of host time.
	telegramCycleBudget = 100_000_000

	telegramMaxMessage = 3800 // chars per message, under the 4096 limit
)

type telegramFrontend struct {
	dataDir     string
	cycleBudget uint64
	locks       sync.Map // one mutex per user
}

func newTelegramFrontend(dataDir string) *telegramFrontend {
	return &telegramFrontend{
		dataDir:     dataDir,
		cycleBudget: telegramCycleBudget,
	}
}

func runTelegram(token string, dataDir string) error {
	if token == "" {
		return fmt.Errorf("a bot token is needed, use -token or the TELEGRAM_TOKEN environment variable")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	tf := newTelegramFrontend(dataDir)

	b, err := bot.New(token, bot.WithDefaultHandler(tf.handler))
	if err != nil {
		return err
	}

	// Register the meta commands, suggested when the user types "/"
	commands := make([]models.BotCommand, 0, len(telegramCommands))
	for _, cmd := range telegramCommands {
		commands = append(commands, models.BotCommand{
			Command:     cmd.command,
			Description: cmd.description,
		})
	}
	ok, err := b.SetMyCommands(context.Background(), &bot.SetMyCommandsParams{
		Commands: commands,
	})
	if err != nil || !ok {
		fmt.Fprintf(os.Stderr, "Error registering the commands menu: %v\n", err)
	} else {
		fmt.Printf("Registered %v commands on the telegram menu\n", len(commands))
	}

	fmt.Println("izapplebasic - Applesoft BASIC telegram bot, https://github.com/ivanizag/izapplebasic")
	fmt.Printf("(data directory %s)\n", dataDir)
	b.Start(context.Background())
	return nil
}

func (tf *telegramFrontend) handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}
	msg := update.Message
	username := msg.From.Username
	if username == "" {
		username = msg.From.FirstName
	}

	// One message at a time per user
	lock, _ := tf.locks.LoadOrStore(msg.From.ID, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	segments, images, err := tf.processMessage(msg.From.ID, username, msg.Text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing a message of %s: %v\n", username, err)
		segments = []outSegment{{false, "internal error, sorry"}}
	}

	for _, segment := range segments {
		text := cleanText(segment.text)
		if text == "" {
			continue
		}
		for _, chunk := range chunkString(text, telegramMaxMessage) {
			params := &bot.SendMessageParams{
				ChatID: msg.Chat.ID,
				Text:   chunk,
			}
			if segment.mono {
				// Only the output of the emulated machine is
				// shown monospaced
				params.Text = "<pre>" + html.EscapeString(chunk) + "</pre>"
				params.ParseMode = models.ParseModeHTML
			}
			_, err = b.SendMessage(ctx, params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error sending a message to %s: %v\n", username, err)
			}
		}
	}

	for _, snapshot := range images {
		var buf bytes.Buffer
		err := png.Encode(&buf, snapshot)
		if err != nil {
			continue
		}
		_, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID: msg.Chat.ID,
			Photo:  &models.InputFileUpload{Filename: "screen.png", Data: &buf},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sending a photo to %s: %v\n", username, err)
		}
	}
}

// processMessage runs the lines of a message on the emulated machine
// of the user, persisting the state before and after.
func (tf *telegramFrontend) processMessage(userID int64, username string, text string) ([]outSegment, []image.Image, error) {
	dir := filepath.Join(tf.dataDir, strconv.FormatInt(userID, 10))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, err
	}
	stateFilename := filepath.Join(dir, "state")

	con := newConsoleTelegram(dir, text)
	env, err := iz.NewEnvironment(nil)
	if err != nil {
		return nil, nil, err
	}
	con.env = env
	con.loadTapePointer()
	env.SetConsole(con)

	if f, err := os.Open(stateFilename); err == nil {
		err = env.LoadState(f)
		f.Close()
		if err != nil {
			return nil, nil, err
		}
	}

	cycles := env.Cycles()
	env.BreakCycles = cycles + tf.cycleBudget
	env.MaxCycles = cycles + 2*tf.cycleBudget
	env.ClearGraphicsDirty()

	env.Run()
	con.saveTapePointer()

	f, err := os.Create(stateFilename)
	if err != nil {
		return nil, nil, err
	}
	err = env.SaveState(f)
	f.Close()
	if err != nil {
		return nil, nil, err
	}

	// Attach a snapshot when the machine drew graphics
	if env.GraphicsDirty() {
		con.images = append(con.images, env.Snapshot())
	}

	tf.logInteraction(dir, username, text, con.text(), len(con.images))
	return con.segments, con.images, nil
}

// logInteraction appends the exchange to the log of the user.
func (tf *telegramFrontend) logInteraction(dir string, username string, input string, output string, images int) {
	f, err := os.OpenFile(filepath.Join(dir, "log.txt"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "=== %s %s\n", time.Now().Format(time.RFC3339), username)
	for _, line := range strings.Split(input, "\n") {
		fmt.Fprintf(f, "< %s\n", line)
	}
	output = strings.TrimRight(output, " \n")
	if output != "" {
		for _, line := range strings.Split(output, "\n") {
			fmt.Fprintf(f, "> %s\n", line)
		}
	}
	if images > 0 {
		fmt.Fprintf(f, "[%d image(s)]\n", images)
	}
}

/*
cleanText prepares a piece of output to be sent: the control chars
like the BELL are removed, as Telegram strips them anyway and
rejects messages that end up empty, like the lone bell of a boot.
*/
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' {
			return -1
		}
		return r
	}, s)
	// Drop empty lines around the text, but keep the leading spaces
	// of the first line: the column alignment matters
	s = strings.TrimRight(s, " \n")
	return strings.TrimLeft(s, "\n")
}

// chunkString splits a string in chunks of at most size chars,
// preferring to cut at line ends.
func chunkString(s string, size int) []string {
	var chunks []string
	for len(s) > size {
		// A newline just after the limit also works as a cut point
		cut := strings.LastIndexByte(s[:size+1], '\n')
		if cut < 1 {
			cut = size
		}
		chunks = append(chunks, s[:cut])
		s = strings.TrimLeft(s[cut:], "\n")
	}
	return append(chunks, s)
}
