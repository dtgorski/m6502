# 6502 CPU emulator

Lightweight cycle-accurate MOS 6502 CPU emulator library for Go.

### Installation

```bash
go get -u github.com/dtgorski/m6502
```
### Cycle accurate timing regime

```go
package main

import (
	"log"
	"time"
	"github.com/dtgorski/m6502"
)

type Bus struct{ mem [0x10000]byte }

func (b *Bus) Read(l, h byte) byte   { return b.mem[uint16(h)<<8|uint16(l)] }
func (b *Bus) Write(l, h, data byte) { b.mem[uint16(h)<<8|uint16(l)] = data }

func main() {
	var err error
	cycles := uint(0)

	bus := &Bus{}
	cpu := m6502.New(bus)

	clock := time.NewTicker(time.Second / 1_000_000 /* Hz */)
	defer clock.Stop()

	for {
		select {
		case <-clock.C:
			if cycles > 0 {
				cycles--
				continue
			}
			if cycles, err = cpu.Step(); err != nil {
				log.Fatal(err)
			}
		}
	}
}
```

### @dev
Try ```make```:
```
$ make

 make help       Displays this list
 make clean      Removes build/test artifacts
 make test       Runs tests with -race (pick: ARGS="-run=<Name>")
 make sniff      Checks format and runs linter (void on success)
 make tidy       Formats source files, cleans go.mod

 Usage: make <TARGET> [ARGS=...]
```
### License
[MIT](https://opensource.org/licenses/MIT) - © dtg [at] lengo [dot] org