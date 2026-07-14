# izapplebasic

Applesoft BASIC on modern hardware. It runs the unmodified Apple II+
ROM on the [iz6502](https://github.com/ivanizag/iz6502) emulated CPU
and intercepts the monitor calls to use the modern computer
interfaces, in the spirit of [bbz](https://github.com/ivanizag/bbz).

The project is a Go library with the emulation core, package
`izapplebasic` at the root, and two frontends in `cmd`: the command
line `izapplebasic` and the `izapplebasic-telegram` bot. All the
filesystem access happens on the frontends.

```
go install github.com/ivanizag/izapplebasic/cmd/...@latest
```

## Command line usage

```
izapplebasic
```

The Apple II+ ROM is embedded in the binary. Another ROM image
(12 KB, 0xd000 to 0xffff) can be used with the `-rom` argument.

```
]10 FOR I=1 TO 5
]20 PRINT I, I*I
]30 NEXT
]RUN
1               1
2               4
3               9
4               16
5               25
```

Options:

- `-rom <file>`: filename of the Apple II+ ROM (default: embedded)
- `-home`: clear the host screen on HOME (default: HOME is ignored)
- `-c`: trace the CPU execution
- `-m`: trace the intercepted monitor calls excluding char output
- `-M`: trace the intercepted monitor calls including char output
- `-s`: trace the accesses to the softswitches at 0xc0xx
- `-l`: do not convert the input to uppercase
- `-r`: disable the readline like input with history
- `-load <file>`: load the emulation state from a file on startup
- `-tape <name>`: name of the cassette tape inserted on startup
  (default: `default`)

## How it works

The emulated machine is an Apple II+ with 48 KB of RAM and nothing in
the slots. On reset, the autostart monitor finds no bootable card and
falls back to Applesoft BASIC.

The monitor entry points for console I/O are patched with an RTS and
intercepted when the program counter reaches them:

- `COUT1` (0xfdf0): char output
- `KEYIN` (0xfd1b): char input, used by `GET`
- `GETLN1` (0xfd6f): line input, used by the direct mode prompt and
  `INPUT`. The prompt printing entry points GETLNZ and GETLN are
  real ROM code running on the intercepted COUT.
- `HOME` (0xfc58): clear the screen, on the command line ignored
  unless `-home` is given
- `WRITE` and `READ` (0xfecd, 0xfefd): the cassette tape

On a real Apple II the screen is random access: Applesoft moves the
cursor for `HTAB` and the comma print zones just by changing `CH`
(0x24). The output here is a stream, so the interception tracks the
host column and pads with spaces when `CH` jumps forward.

The intercepted output is also mirrored to the text page memory at
0x0400, respecting the text window, so the screen snapshots show the
session as a real Apple II would.

## Meta commands

Lines starting with `/` are processed by the frontend, they never
reach the emulated machine. On the command line:

- `/help`: list the meta commands
- `/quit`: exit
- `/save [filename]` and `/load [filename]`: save the emulation
  state, CPU, RAM and video mode, and load it back, even on another
  session. The state can also be loaded on startup with the `-load`
  argument.
- `/screenshot [filename.png]`: save a PNG image of the emulated screen,
  rendered with the [izapple2](https://github.com/ivanizag/izapple2)
  screen module. The video mode softswitches (0xc050-0xc057) are
  tracked, so `GR` and `HGR` graphics, mixed modes and page 2 are
  rendered as they would show on the real screen.
- `/tape [name]` and `/rewind [block]`: manage the emulated cassette
  deck, see below.

## The cassette

The monitor tape routines, `WRITE` (0xfecd) and `READ` (0xfefd), are
intercepted too, so `SAVE`, `LOAD`, `STORE`, `RECALL` and `SHLOAD`
work. Each monitor call is one checksummed record on a real tape,
here one block stored as the file `tape-NAME-nn.tape` with the raw
bytes. Reads and writes happen at the current tape position and
advance it, `/tape name` inserts another tape rewound to block 0 and
`/rewind [block]` moves the position. An Applesoft `SAVE` writes two
blocks, the length header and the program:

```
]/tape adventure
tape adventure inserted and rewound
]10 PRINT "YOU ARE IN A MAZE"
]SAVE
]/rewind
tape adventure at block 0
]LOAD
]RUN
YOU ARE IN A MAZE
```

The tape operations are silent as on the real machine, the `-m`
trace shows the blocks being read and written. Reading past the
last block or a block recorded with a different size shows the
monitor `ERR`, as a bad tape would.

## Frontends

- Command line: readline like input with history recall on the up
  and down arrows, saved in `.izapplebasichistory`. Falls back to
  plain stdin/stdout when the input is piped or with `-r`.
- Telegram: see below.

The frontends implement the `Console` interface of the core package,
including their own meta commands: the telegram bot has no way to
name files on the host.

## Telegram bot

```
izapplebasic-telegram -token <bot token>
```

The token comes from [@BotFather](https://t.me/BotFather), by
argument or in the `TELEGRAM_TOKEN` environment variable. The bot
connects to Telegram with long polling, no webhook or open port is
needed.

Every user gets a persistent Apple II: on each message the state is
loaded from a file, the lines of the message are executed and the
state is saved back. Multiline messages work, a whole program can be
pasted at once. A program waiting on `INPUT` or `GET` takes the next
message as the answer. Programs running longer than the per message
budget are stopped with the control-C break.

Text replies are monospace. When a command draws GR or HGR graphics
a snapshot of the screen is attached.

The bot has its own meta commands, registered on Telegram so they
are suggested when typing `/`:

- `/help`: list the commands
- `/screenshot`: show an image of the emulated screen
- `/reset`: reboot the machine, losing everything not saved
- `/save [name]`, `/load [name]`, `/list` and `/delete [name]`:
  manage named states, stored on the folder of the user, up to 30
  per user
- `/tape [name]`, `/rewind [block]` and `/deletetape <name>`: the
  cassette deck for the BASIC `SAVE` and `LOAD`, stored on the
  folder of the user, up to 30 blocks per user. The inserted tape
  and its position persist between messages.

Each user has a directory under `-data` (default `telegram-data`)
with the current state, the named saved states and a log of the
interaction.

To deploy with docker compose, from `cmd/izapplebasic-telegram`: put
the token in a `.env` file (`TELEGRAM_TOKEN=...`), create the
`telegram-data` directory owned by uid 1000, and run
`docker compose up -d`.

## Limitations

- Line based input: `GET` waits for the enter key and takes the
  characters of the line one by one.
- The graphics are only visible through `/screenshot`, there is no
  live graphics display. No disk.
- Control-C breaks the running BASIC program as on a real Apple II,
  press it twice in fast succession to quit.
