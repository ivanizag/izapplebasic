# izapplebasic

Applesoft BASIC on modern hardware. It runs the unmodified Apple II+
ROM on the [iz6502](https://github.com/ivanizag/iz6502) emulated CPU
and intercepts the monitor calls to use the modern computer
interfaces, in the spirit of [bbz](https://github.com/ivanizag/bbz).

## Usage

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

## How it works

The emulated machine is an Apple II+ with 48 KB of RAM and nothing in
the slots. On reset, the autostart monitor finds no bootable card and
falls back to Applesoft BASIC.

The monitor entry points for console I/O are patched with an RTS and
intercepted when the program counter reaches them:

- `COUT1` (0xfdf0): char output
- `KEYIN` (0xfd1b): char input, used by `GET`
- `GETLNZ`, `GETLN`, `GETLN1` (0xfd67, 0xfd6a, 0xfd6f): line input,
  used by the direct mode prompt and `INPUT`
- `HOME` (0xfc58): clear the screen, ignored unless `-home` is given

On a real Apple II the screen is random access: Applesoft moves the
cursor for `HTAB` and the comma print zones just by changing `CH`
(0x24). The output here is a stream, so the interception tracks the
host column and pads with spaces when `CH` jumps forward.

The intercepted output is also mirrored to the text page memory at
0x0400, respecting the text window, so the screen snapshots show the
session as a real Apple II would.

## Meta commands

Lines starting with `/` are processed on the host, they never reach
the emulated machine:

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

## Frontends

- Command line: readline like input with history recall on the up
  and down arrows, saved in `.izapplebasichistory`. Falls back to
  plain stdin/stdout when the input is piped or with `-r`.
- Telegram: planned.

The frontends implement the `console` interface in `console.go`.

## Limitations

- Line based input: `GET` waits for the enter key and takes the
  characters of the line one by one.
- The graphics are only visible through `/screenshot`, there is no
  live graphics display. No cassette, no disk.
- Control-C breaks the running BASIC program as on a real Apple II,
  press it twice in fast succession to quit.
