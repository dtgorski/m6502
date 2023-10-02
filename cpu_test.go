// MIT License · Daniel T. Gorski · dtg [at] lengo [dot] org · 09/2023

package m6502

import (
	"errors"
	"io"
	"os"
	"runtime"
	"testing"
)

type memoryBus struct{ mem [0x10000]byte }

func (m *memoryBus) Read(l, h byte) byte {
	return m.mem[uint16(h)<<8|uint16(l)]
}
func (m *memoryBus) Write(l, h, data byte) {
	m.mem[uint16(h)<<8|uint16(l)] = data
}
func (m *memoryBus) Reset() {
	for i, n := 0, len(m.mem); i < n; i++ {
		m.mem[i] = 0x00
	}
}

func TestCPU(t *testing.T) {

	bus := &memoryBus{}
	cpu := New(bus)

	// Aliases
	A := func(b byte) { cpu.a = b }                // Set A
	X := func(b byte) { cpu.x = b }                // Set X
	Y := func(b byte) { cpu.y = b }                // Set Y
	F := func(f flag) { cpu.p.set(true, f) }       // Set Flag
	H := func(f flag) bool { return cpu.p.has(f) } // Has Flag?
	R := bus.Read                                  // Read
	W := func(l, h byte, a ...byte) {              // Write
		for _, b := range a {
			bus.Write(l, h, b)
			if l++; l == 0 {
				h++
			}
		}
	}
	EQ := func(a, b byte) {
		if a != b {
			_, _, l, _ := runtime.Caller(1)
			t.Errorf("unexpected, want 0x%02x, got 0x%02x in line %d", a, b, l)
		}
	}
	EX := func(c bool) {
		if !c {
			_, _, l, _ := runtime.Caller(1)
			t.Errorf("unexpected 'not equal' in line %d", l)
		}
	}

	type test struct {
		init func() // pre-test setup function
		mne  string // mnemonic for error reporting
		mem  []byte // instruction bytes
		cost uint   // expected cycle cost
		post func() // post-test verification function
	}

	tests := [0x100][]test{}

	//  * add 1 to cycles if page boundary is crossed
	// ** add 1 to cycles if branch occurs on same page
	// ** add 2 to cycles if branch occurs to different page

	tests[0x00 /* BRK | implied | N- Z- C- I- D- V- | 7 */] = []test{
		{
			func() { W(0xFE, 0xFF, 0x12, 0x34) },
			"BRK", []byte{0x00}, 7,
			func() { EQ(0x12, cpu.PCL()); EQ(0x34, cpu.PCH()) },
		},
	}
	tests[0x20 /* JSR oper | absolute | N- Z- C- I- D- V- | 6 */] = []test{
		{
			func() {},
			"JSR", []byte{0x20, 0x12, 0x34}, 6,
			func() { EQ(0x12, cpu.PCL()); EQ(0x02, R(0xFE, 0x01)) },
		},
	}
	tests[0x40 /* RTI | implied | from stack | 7 */] = []test{
		{
			func() { W(0xFD, 0x01, 0xFF, 0x12, 0x34); cpu.s -= 3 },
			"RTI", []byte{0x40}, 7,
			func() { EQ(0x12, cpu.PCL()); EQ(0x34, cpu.PCH()); EQ(0xCF, byte(*cpu.p)) },
		},
	}
	tests[0x60 /* RTS | implied | N- Z- C- I- D- V- | 6 */] = []test{
		{
			func() { W(0xFE, 0x01, 0x11, 0x34); cpu.s -= 2 },
			"RTS", []byte{0x60}, 6,
			func() { EQ(0x12, cpu.PCL()); EQ(0x34, cpu.PCH()); EQ(0xFF, cpu.s) },
		},
	}
	tests[0x80 /* NOP | immediate | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0x80}, 2, func() {},
		},
	}
	tests[0xA0 /* LDY #oper | immediate | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() {},
			"LDY", []byte{0xA0, 0x80}, 2,
			func() { EQ(0x80, cpu.y); EX(H(flagN)) },
		},
	}
	tests[0xC0 /* CPY #oper | immediate | N+ Z+ C+ I- D- V- | 2 */] = []test{
		{
			func() { Y(0x80) },
			"CPY", []byte{0xC0, 0x80}, 2,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { Y(0x81) },
			"CPY", []byte{0xC0, 0x80}, 2,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { Y(0x81) },
			"CPY", []byte{0xC0, 0x01}, 2,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { Y(0x01) },
			"CPY", []byte{0xC0, 0x80}, 2,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { Y(0x01) },
			"CPY", []byte{0xC0, 0x88}, 2,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xE0 /* CPX #oper | immediate | N+ Z+ C+ I- D- V- | 2 */] = []test{
		{
			func() { X(0x80) },
			"CPX", []byte{0xE0, 0x80}, 2,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { X(0x81) },
			"CPX", []byte{0xE0, 0x80}, 2,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { X(0x81) },
			"CPX", []byte{0xE0, 0x01}, 2,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { X(0x01) },
			"CPX", []byte{0xE0, 0x80}, 2,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { X(0x01) },
			"CPX", []byte{0xE0, 0x88}, 2,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}

	// ---

	tests[0x01 /* ORA (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- | 6 */] = []test{
		{
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); X(0x08); A(0x01) },
			"ORA", []byte{0x01, 0x08}, 6,
			func() { EQ(0x81, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x21 /* AND (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- | 6 */] = []test{
		{
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); X(0x08); A(0x81) },
			"AND", []byte{0x21, 0x08}, 6,
			func() { EQ(0x80, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x41 /* EOR (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- | 6 */] = []test{
		{
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); X(0x08); A(0x81) },
			"EOR", []byte{0x41, 0x08}, 6,
			func() { EQ(0x01, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}

	tests[0x61 /* ADC (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V+ | 6 */] = []test{
		{
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); X(0x08); A(0x81) },
			"ADC", []byte{0x61, 0x08}, 6,
			func() { EQ(0x01, cpu.a); EX(H(flagC)); EX(cpu.p.has(flagV)) },
		},
	}
	tests[0x81 /* STA (oper,X) | (indirect,X) | N- Z- C- I- D- V- | 6 */] = []test{
		{
			func() { W(0x10, 0x00, 0x12, 0x34); X(0x08); A(0x81) },
			"STA", []byte{0x81, 0x08}, 6,
			func() { EQ(0x81, R(0x12, 0x34)) },
		},
	}
	tests[0xA1 /* LDA (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- | 6 */] = []test{
		{
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); X(0x08) },
			"LDA", []byte{0xA1, 0x08}, 6,
			func() { EQ(0x80, cpu.a) },
		},
	}
	tests[0xC1 /* CMP (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); X(0x08); A(0x80) },
			"CMP", []byte{0xC1, 0x08}, 6,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); X(0x08); A(0x81) },
			"CMP", []byte{0xC1, 0x08}, 6,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x01); X(0x08); A(0x81) },
			"CMP", []byte{0xC1, 0x08}, 6,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); X(0x08); A(0x01) },
			"CMP", []byte{0xC1, 0x08}, 6,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x88); X(0x08); A(0x01) },
			"CMP", []byte{0xC1, 0x08}, 6,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xE1 /* SBC (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V+ | 6 */] = []test{
		{
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); A(0x80); X(0x08) },
			"SBC", []byte{0xE1, 0x08}, 6,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); A(0x80); X(0x08); F(flagC) },
			"SBC", []byte{0xE1, 0x08}, 6,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); A(0x90); X(0x08); F(flagD) },
			"SBC", []byte{0xE1, 0x08}, 6,
			func() { EQ(0x09, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x10, 0x00, 0x12, 0x34); W(0x12, 0x34, 0x80); A(0x90); X(0x08); F(flagC | flagD) },
			"SBC", []byte{0xE1, 0x08}, 6,
			func() { EQ(0x10, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}

	// ---

	tests[0x02 /* HLT */] = []test{{func() {}, "HLT", []byte{0x02}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0x22 /* HLT */] = []test{{func() {}, "HLT", []byte{0x22}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0x42 /* HLT */] = []test{{func() {}, "HLT", []byte{0x42}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0x62 /* HLT */] = []test{{func() {}, "HLT", []byte{0x62}, 0, func() { EX(cpu.error == ErrHalted) }}}

	tests[0x82 /* NOP | immediate | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0x82}, 2, func() {},
		},
	}
	tests[0xA2 /* LDX #oper | immediate | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() {},
			"LDX", []byte{0xA2, 0x00}, 2,
			func() { EQ(0x00, cpu.x); EX(!H(flagN)); EX(H(flagZ)) },
		}, {
			func() {},
			"LDX", []byte{0xA2, 0x20}, 2,
			func() { EQ(0x20, cpu.x); EX(!H(flagN)); EX(!H(flagZ)) },
		}, {
			func() {},
			"LDX", []byte{0xA2, 0xE0}, 2,
			func() { EQ(0xE0, cpu.x); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xC2 /* NOP | immediate | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0xC2}, 2, func() {},
		},
	}
	tests[0xE2 /* NOP | immediate | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0xE2}, 2, func() {},
		},
	}

	// ---

	tests[0x04 /* NOP | zeropage | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() {}, "NOP", []byte{0x04, 0x00}, 3, func() {},
		},
	}
	tests[0x24 /* BIT oper | zeropage | N+ Z+ C- I- D- V+ | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0xAA); A(0x40) },
			"BIT", []byte{0x24, 0x80}, 3,
			func() { EX(H(flagZ)); EX(H(flagN)); EX(!cpu.p.has(flagV)) },
		}, {
			func() { W(0x80, 0x00, 0x40) },
			"BIT", []byte{0x24, 0x80}, 3,
			func() { EX(H(flagZ)); EX(!H(flagN)); EX(cpu.p.has(flagV)) },
		},
	}
	tests[0x44 /* NOP | zeropage | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() {}, "NOP", []byte{0x44, 0x00}, 3, func() {},
		},
	}
	tests[0x64 /* NOP | zeropage | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() {}, "NOP", []byte{0x64, 0x00}, 3, func() {},
		},
	}
	tests[0x84 /* STY oper | zeropage | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() { Y(0x20) },
			"STY", []byte{0x84, 0x80}, 3,
			func() { EQ(0x20, R(0x80, 0x00)) },
		},
	}
	tests[0xA4 /* LDY oper | zeropage | N+ Z+ C- I- D- V- | 3 */] = []test{
		{
			func() { W(0x20, 0x00, 0x80) },
			"LDY", []byte{0xA4, 0x20}, 3,
			func() { EQ(0x80, cpu.y); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xC4 /* CPY oper | zeropage | N+ Z+ C+ I- D- V- | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); Y(0x80) },
			"CPY", []byte{0xC4, 0x80}, 3,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); Y(0x81) },
			"CPY", []byte{0xC4, 0x80}, 3,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x01); Y(0x81) },
			"CPY", []byte{0xC4, 0x80}, 3,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); Y(0x01) },
			"CPY", []byte{0xC4, 0x80}, 3,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x88); Y(0x01) },
			"CPY", []byte{0xC4, 0x80}, 3,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xE4 /* CPX oper | zeropage | N+ Z+ C+ I- D- V- | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); X(0x80) },
			"CPX", []byte{0xE4, 0x80}, 3,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); X(0x81) },
			"CPX", []byte{0xE4, 0x80}, 3,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x01); X(0x81) },
			"CPX", []byte{0xE4, 0x80}, 3,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); X(0x01) },
			"CPX", []byte{0xE4, 0x80}, 3,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x88); X(0x01) },
			"CPX", []byte{0xE4, 0x80}, 3,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}

	// ---

	tests[0x05 /* ORA oper | zeropage | N+ Z+ C- I- D- V- | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); A(0x01) },
			"ORA", []byte{0x05, 0x80}, 3,
			func() { EQ(0x81, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x25 /* AND oper | zeropage | N+ Z+ C- I- D- V- | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0xAA); A(0x0F) },
			"AND", []byte{0x25, 0x80}, 3,
			func() { EQ(0x0A, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}
	tests[0x45 /* EOR oper | zeropage | N+ Z+ C- I- D- V- | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0xAA); A(0xFF) },
			"EOR", []byte{0x45, 0x80}, 3,
			func() { EQ(0x55, cpu.a); EX(!H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0x65 /* ADC oper | zeropage | N+ Z+ C+ I- D- V+ | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); A(0x80) },
			"ADC", []byte{0x65, 0x80}, 3,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x80); F(flagC) },
			"ADC", []byte{0x65, 0x80}, 3,
			func() { EQ(0x01, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x90); F(flagD) },
			"ADC", []byte{0x65, 0x80}, 3,
			func() { EQ(0x70, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x90); F(flagC | flagD) },
			"ADC", []byte{0x65, 0x80}, 3,
			func() { EQ(0x71, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}
	tests[0x85 /* STA oper | zeropage | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() { A(0x20) },
			"STA", []byte{0x85, 0x80}, 3,
			func() { EQ(0x20, R(0x80, 0x00)) },
		},
	}
	tests[0xA5 /* LDA oper | zeropage | N+ Z+ C- I- D- V- | 3 */] = []test{
		{
			func() { W(0x20, 0x00, 0x80) },
			"LDA", []byte{0xA5, 0x20}, 3,
			func() { EQ(0x80, cpu.a); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xC5 /* CMP oper | zeropage | N+ Z+ C+ I- D- V- | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); A(0x80) },
			"CMP", []byte{0xC5, 0x80}, 3,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x81) },
			"CMP", []byte{0xC5, 0x80}, 3,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x01); A(0x81) },
			"CMP", []byte{0xC5, 0x80}, 3,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x01) },
			"CMP", []byte{0xC5, 0x80}, 3,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x88); A(0x01) },
			"CMP", []byte{0xC5, 0x80}, 3,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xE5 /* SBC oper | zeropage | N+ Z+ C+ I- D- V+ | 3 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); A(0x80) },
			"SBC", []byte{0xE5, 0x80}, 3,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x80); F(flagC) },
			"SBC", []byte{0xE5, 0x80}, 3,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x90); F(flagD) },
			"SBC", []byte{0xE5, 0x80}, 3,
			func() { EQ(0x09, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x90); F(flagC | flagD) },
			"SBC", []byte{0xE5, 0x80}, 3,
			func() { EQ(0x10, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}

	// ---

	tests[0x06 /* ASL oper | zeropage | N+ Z+ C+ I- D- V- | 5 */] = []test{
		{
			func() { W(0x80, 0x00, 0x55) },
			"ASL", []byte{0x06, 0x80}, 5,
			func() { EQ(0xAA, R(0x80, 0x00)); EX(H(flagN)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xAA) },
			"ASL", []byte{0x06, 0x80}, 5,
			func() { EQ(0x54, R(0x80, 0x00)); EX(!H(flagN)); EX(H(flagC)) },
		},
	}
	tests[0x26 /* ROL oper | zeropage | N+ Z+ C+ I- D- V- | 5 */] = []test{
		{
			func() { W(0x80, 0x00, 0x55) },
			"ROL", []byte{0x26, 0x80}, 5,
			func() { EQ(0xAA, R(0x80, 0x00)); EX(H(flagN)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xAA); F(flagC) },
			"ROL", []byte{0x26, 0x80}, 5,
			func() { EQ(0x55, R(0x80, 0x00)); EX(!H(flagN)); EX(H(flagC)) },
		},
	}
	tests[0x46 /* LSR oper | zeropage | N0 Z+ C+ I- D- V- | 5 */] = []test{
		{
			func() { W(0x80, 0x00, 0x55) },
			"LSR", []byte{0x46, 0x80}, 5,
			func() { EQ(0x2A, R(0x80, 0x00)); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xAA) },
			"LSR", []byte{0x46, 0x80}, 5,
			func() { EQ(0x55, R(0x80, 0x00)); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x66 /* ROR oper | zeropage | N+ Z+ C+ I- D- V- | 5 */] = []test{
		{
			func() { W(0x80, 0x00, 0x55) },
			"ROR", []byte{0x66, 0x80}, 5,
			func() { EQ(0x2A, R(0x80, 0x00)); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xAA) },
			"ROR", []byte{0x66, 0x80}, 5,
			func() { EQ(0x55, R(0x80, 0x00)); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x86 /* STX oper | zeropage | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() { X(0xAA) },
			"STX", []byte{0x86, 0x80}, 3,
			func() { EQ(0xAA, R(0x80, 0x00)) },
		},
	}
	tests[0xA6 /* LDX oper | zeropage | N+ Z+ C- I- D- V- | 3 */] = []test{
		{
			func() { W(0x20, 0x00, 0x80) },
			"LDX", []byte{0xA6, 0x20}, 3,
			func() { EQ(0x80, cpu.x); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xC6 /* DEC oper | zeropage | N+ Z+ C- I- D- V- | 5 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80) },
			"DEC", []byte{0xC6, 0x80}, 5,
			func() { EQ(0x7F, R(0x80, 0x00)); EX(!H(flagN)) },
		},
	}
	tests[0xE6 /* INC oper | zeropage | N+ Z+ C- I- D- V- | 5 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80) },
			"INC", []byte{0xE6, 0x80}, 5,
			func() { EQ(0x81, R(0x80, 0x00)); EX(H(flagN)) },
		},
	}

	tests[0x08 /* PHP | implied | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() {},
			"PHP", []byte{0x08}, 3,
			func() { EQ(byte(flagU|flagB), R(0xFF, 0x01)) },
		},
	}
	tests[0x28 /* PLP | implied | from stack | 4 */] = []test{
		{
			func() { W(0xFF, 0x01, 0xFF); cpu.s = 0xFE },
			"PLP", []byte{0x28}, 4,
			func() { EX(H(flagN)); EX(!cpu.p.has(flagB)); EX(!cpu.p.has(flagU)) },
		},
	}
	tests[0x48 /* PHA | implied | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() { A(0x80) },
			"PHA", []byte{0x48}, 3,
			func() { EQ(0x80, R(0xFF, 0x01)) },
		},
	}
	tests[0x68 /* PLA | implied | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0xFF, 0x01, 0x80); cpu.s = 0xFE },
			"PLA", []byte{0x68}, 4,
			func() { EQ(0x80, cpu.a); EX(H(flagN)) },
		},
	}
	tests[0x88 /* DEY | implied | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { Y(0x00) },
			"DEY", []byte{0x88}, 2,
			func() { EQ(0xFF, cpu.y); EX(H(flagN)) },
		},
	}
	tests[0xA8 /* TAY | implied | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { A(0x80) },
			"TAY", []byte{0xA8}, 2,
			func() { EQ(0x80, cpu.y); EX(H(flagN)) },
		},
	}
	tests[0xC8 /* INY | implied | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { Y(0x80) },
			"INY", []byte{0xC8}, 2,
			func() { EQ(0x81, cpu.y); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xE8 /* INX | implied | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { X(0x80) },
			"INX", []byte{0xE8}, 2,
			func() { EQ(0x81, cpu.x); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}

	// ---

	tests[0x09 /* ORA #oper | immediate | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { A(0x01) },
			"ORA", []byte{0x09, 0x80}, 2,
			func() { EQ(0x81, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x29 /* AND #oper | immediate | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { A(0x0F) },
			"AND", []byte{0x29, 0xAA}, 2,
			func() { EQ(0x0A, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}
	tests[0x49 /* EOR #oper | immediate | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { A(0xFF) },
			"EOR", []byte{0x49, 0xAA}, 2,
			func() { EQ(0x55, cpu.a); EX(!H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0x69 /* ADC #oper | immediate | N+ Z+ C+ I- D- V+ | 2 */] = []test{
		{
			func() { A(0x80) },
			"ADC", []byte{0x69, 0x80}, 2,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x80); F(flagC) },
			"ADC", []byte{0x69, 0x80}, 2,
			func() { EQ(0x01, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x90); F(flagD) },
			"ADC", []byte{0x69, 0x80}, 2,
			func() { EQ(0x70, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x90); F(flagC | flagD) },
			"ADC", []byte{0x69, 0x80}, 2,
			func() { EQ(0x71, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}
	tests[0x89 /* NOP | immediate | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0x89}, 2, func() {},
		},
	}
	tests[0xA9 /* LDA #oper | immediate | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() {},
			"LDA", []byte{0xA9, 0x20}, 2,
			func() { EQ(0x20, cpu.a); EX(!H(flagN)); EX(!H(flagZ)) },
		}, {
			func() {},
			"LDA", []byte{0xA9, 0xE0}, 2,
			func() { EQ(0xE0, cpu.a); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xC9 /* CMP #oper | immediate | N+ Z+ C+ I- D- V- | 2 */] = []test{
		{
			func() { A(0x80) },
			"CMP", []byte{0xC9, 0x80}, 2,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x81) },
			"CMP", []byte{0xC9, 0x80}, 2,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x81) },
			"CMP", []byte{0xC9, 0x01}, 2,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x01) },
			"CMP", []byte{0xC9, 0x80}, 2,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { A(0x01) },
			"CMP", []byte{0xC9, 0x88}, 2,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xE9 /* SBC #oper | immediate | N+ Z+ C+ I- D- V+ | 2 */] = []test{
		{
			func() { A(0x80) },
			"SBC", []byte{0xE9, 0x80}, 2,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { A(0x80); F(flagC) },
			"SBC", []byte{0xE9, 0x80}, 2,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x90); F(flagD) },
			"SBC", []byte{0xE9, 0x80}, 2,
			func() { EQ(0x09, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x90); F(flagC | flagD) },
			"SBC", []byte{0xE9, 0x80}, 2,
			func() { EQ(0x10, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}

	// ---

	tests[0x0A /* ASL A | accumulator | N+ Z+ C+ I- D- V- | 2 */] = []test{
		{
			func() { A(0xAA) },
			"ASL", []byte{0x0A}, 2,
			func() { EQ(0x54, cpu.a); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { A(0x07) },
			"ASL", []byte{0x0A}, 2,
			func() { EQ(0x0E, cpu.a); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0x2A /* ROL A | accumulator | N+ Z+ C+ I- D- V- | 2 */] = []test{
		{
			func() { A(0xAA); F(flagC) },
			"ROL", []byte{0x2A}, 2,
			func() { EQ(0x55, cpu.a); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { A(0xAA); cpu.p.set(false, flagC) },
			"ROL", []byte{0x2A}, 2,
			func() { EQ(0x54, cpu.a); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { A(0x07) },
			"ROL", []byte{0x2A}, 2,
			func() { EQ(0x0E, cpu.a); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x4A /* LSR A | accumulator | N0 Z+ C+ I- D- V- | 2 */] = []test{
		{
			func() { A(0xAA) },
			"LSR", []byte{0x4A}, 2,
			func() { EQ(0x55, cpu.a); EX(!H(flagN)); EX(!H(flagC)) },
		}, {
			func() { A(0x07) },
			"LSR", []byte{0x4A}, 2,
			func() { EQ(0x03, cpu.a); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}
	tests[0x6A /* ROR A | accumulator | N+ Z+ C+ I- D- V- | 2 */] = []test{
		{
			func() { A(0x55) },
			"ROR", []byte{0x6A}, 2,
			func() { EQ(0x2A, cpu.a); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { A(0xAA) },
			"ROR", []byte{0x6A}, 2,
			func() { EQ(0x55, cpu.a); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x8A /* TXA | implied | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { X(0x80) },
			"TXA", []byte{0x8A}, 2,
			func() { EQ(0x80, cpu.a); EX(H(flagN)); EX(!H(flagZ)) },
		}, {
			func() { X(0x20) },
			"TXA", []byte{0x8A}, 2,
			func() { EQ(0x20, cpu.a); EX(!H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xAA /* TAX | implied | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { A(0x80) },
			"TAX", []byte{0xAA}, 2,
			func() { EQ(0x80, cpu.x); EX(H(flagN)); EX(!H(flagZ)) },
		}, {
			func() { A(0x20) },
			"TAX", []byte{0xAA}, 2,
			func() { EQ(0x20, cpu.x); EX(!H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xCA /* DEX | implied  | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { X(0x00) },
			"DEX", []byte{0xCA}, 2,
			func() { EQ(0xFF, cpu.x); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xEA /* NOP | implied | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0xEA}, 2, func() {},
		},
	}

	tests[0x0C /* NOP | absolute | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() {}, "NOP", []byte{0x0C, 0x00, 0x00}, 4, func() {},
		},
	}
	tests[0x2C /* BIT oper | absolute | N+ Z+ C- I- D- V+ | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0xAA); A(0x40) },
			"BIT", []byte{0x2C, 0x12, 0x34}, 4,
			func() { EX(H(flagZ)); EX(H(flagN)); EX(!cpu.p.has(flagV)) },
		}, {
			func() { W(0x12, 0x34, 0x40) },
			"BIT", []byte{0x2C, 0x12, 0x34}, 4,
			func() { EX(H(flagZ)); EX(!H(flagN)); EX(cpu.p.has(flagV)) },
		},
	}
	tests[0x4C /* JMP oper | absolute | N- Z- C- I- D- V- | 3 */] = []test{
		{
			func() {},
			"JMP", []byte{0x4C, 0x12, 0x34}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x34, cpu.PCH()) },
		},
	}
	tests[0x6C /* JMP (oper) | indirect | N- Z- C- I- D- V- | 5 */] = []test{
		{
			func() { W(0xFF, 0x80, 0xAA); W(0x00, 0x80, 0x55) },
			"JMP", []byte{0x6C, 0xFF, 0x80}, 5,
			func() { EQ(0xAA, cpu.PCL()); EQ(0x55, cpu.PCH()) },
		},
	}
	tests[0x8C /* STY oper | absolute | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() { Y(0x80) },
			"STY", []byte{0x8C, 0x12, 0x34}, 4,
			func() { EQ(0x80, R(0x12, 0x34)) },
		},
	}
	tests[0xAC /* LDY oper | absolute | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80) },
			"LDY", []byte{0xAC, 0x12, 0x34}, 4,
			func() { EQ(0x80, cpu.y); EX(H(flagN)); EX(!H(flagZ)) },
		}, {
			func() { W(0x12, 0x34, 0x20) },
			"LDY", []byte{0xAC, 0x12, 0x34}, 4,
			func() { EQ(0x20, cpu.y); EX(!H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xCC /* CPY oper | absolute | N+ Z+ C+ I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); Y(0x80) },
			"CPY", []byte{0xCC, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); Y(0x81) },
			"CPY", []byte{0xCC, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x01); Y(0x81) },
			"CPY", []byte{0xCC, 0x12, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); Y(0x01) },
			"CPY", []byte{0xCC, 0x12, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x88); Y(0x01) },
			"CPY", []byte{0xCC, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xEC /* CPX oper | absolute | N+ Z+ C+ I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); X(0x80) },
			"CPX", []byte{0xEC, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); X(0x81) },
			"CPX", []byte{0xEC, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x01); X(0x81) },
			"CPX", []byte{0xEC, 0x12, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); X(0x01) },
			"CPX", []byte{0xEC, 0x12, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x88); X(0x01) },
			"CPX", []byte{0xEC, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}

	// ---

	tests[0x0D /* ORA oper | absolute | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); A(0x01) },
			"ORA", []byte{0x0D, 0x12, 0x34}, 4,
			func() { EQ(0x81, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x2D /* AND oper | absolute | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0xAA); A(0x0F) },
			"AND", []byte{0x2D, 0x12, 0x34}, 4,
			func() { EQ(0x0A, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}
	tests[0x4D /* EOR oper | absolute | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0xAA); A(0x0F) },
			"EOR", []byte{0x4D, 0x12, 0x34}, 4,
			func() { EQ(0xA5, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x6D /* ADC oper | absolute | N+ Z+ C+ I- D- V+ | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); A(0x80) },
			"ADC", []byte{0x6D, 0x12, 0x34}, 4,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x80); F(flagC) },
			"ADC", []byte{0x6D, 0x12, 0x34}, 4,
			func() { EQ(0x01, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x90); F(flagD) },
			"ADC", []byte{0x6D, 0x12, 0x34}, 4,
			func() { EQ(0x70, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x90); F(flagC | flagD) },
			"ADC", []byte{0x6D, 0x12, 0x34}, 4,
			func() { EQ(0x71, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}
	tests[0x8D /* STA oper | absolute | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() { A(0x80) },
			"STA", []byte{0x8D, 0x12, 0x34}, 4,
			func() { EQ(0x80, R(0x12, 0x34)) },
		},
	}
	tests[0xAD /* LDA oper | absolute | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x20) },
			"LDA", []byte{0xAD, 0x12, 0x34}, 4,
			func() { EQ(0x20, cpu.a) },
		},
	}
	tests[0xCD /* CMP oper | absolute | N+ Z+ C+ I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); A(0x80) },
			"CMP", []byte{0xCD, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x81) },
			"CMP", []byte{0xCD, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x01); A(0x81) },
			"CMP", []byte{0xCD, 0x12, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x01) },
			"CMP", []byte{0xCD, 0x12, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x88); A(0x01) },
			"CMP", []byte{0xCD, 0x12, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xED /* SBC oper | absolute | N+ Z+ C+ I- D- V+ | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); A(0x80) },
			"SBC", []byte{0xED, 0x12, 0x34}, 4,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x80); F(flagC) },
			"SBC", []byte{0xED, 0x12, 0x34}, 4,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x90); F(flagD) },
			"SBC", []byte{0xED, 0x12, 0x34}, 4,
			func() { EQ(0x09, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x90); F(flagC | flagD) },
			"SBC", []byte{0xED, 0x12, 0x34}, 4,
			func() { EQ(0x10, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}

	// ---

	tests[0x0E /* ASL oper | absolute | N+ Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x12, 0x34, 0x55) },
			"ASL", []byte{0x0E, 0x12, 0x34}, 6,
			func() { EQ(0xAA, R(0x12, 0x34)); EX(H(flagN)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0xAA) },
			"ASL", []byte{0x0E, 0x12, 0x34}, 6,
			func() { EQ(0x54, R(0x12, 0x34)); EX(!H(flagN)); EX(H(flagC)) },
		},
	}
	tests[0x2E /* ROL oper | absolute | N+ Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x12, 0x34, 0x55) },
			"ROL", []byte{0x2E, 0x12, 0x34}, 6,
			func() { EQ(0xAA, R(0x12, 0x34)); EX(H(flagN)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0xAA); F(flagC) },
			"ROL", []byte{0x2E, 0x12, 0x34}, 6,
			func() { EQ(0x55, R(0x12, 0x34)); EX(!H(flagN)); EX(H(flagC)) },
		},
	}
	tests[0x4E /* LSR oper | absolute | N0 Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x12, 0x34, 0x55) },
			"LSR", []byte{0x4E, 0x12, 0x34}, 6,
			func() { EQ(0x2A, R(0x12, 0x34)); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0xAA) },
			"LSR", []byte{0x4E, 0x12, 0x34}, 6,
			func() { EQ(0x55, R(0x12, 0x34)); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x6E /* ROR oper | absolute | N+ Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x12, 0x34, 0x55) },
			"ROR", []byte{0x6E, 0x12, 0x34}, 6,
			func() { EQ(0x2A, R(0x12, 0x34)); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0xAA) },
			"ROR", []byte{0x6E, 0x12, 0x34}, 6,
			func() { EQ(0x55, R(0x12, 0x34)); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x8E /* STX oper | absolute | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() { X(0x80) },
			"STX", []byte{0x8E, 0x12, 0x34}, 4,
			func() { EQ(0x80, R(0x12, 0x34)) },
		},
	}
	tests[0xAE /* LDX oper | absolute | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80) },
			"LDX", []byte{0xAE, 0x12, 0x34}, 4,
			func() { EQ(0x80, cpu.x); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xCE /* DEC oper | absolute | N+ Z+ C- I- D- V- | 6 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80) },
			"DEC", []byte{0xCE, 0x12, 0x34}, 6,
			func() { EQ(0x7F, R(0x12, 0x34)); EX(!H(flagN)) },
		},
	}
	tests[0xEE /* INC oper | absolute | N+ Z+ C- I- D- V- | 6  */] = []test{
		{
			func() { W(0x12, 0x34, 0x80) },
			"INC", []byte{0xEE, 0x12, 0x34}, 6,
			func() { EQ(0x81, R(0x12, 0x34)); EX(H(flagN)) },
		},
	}

	tests[0x10 /* BPL oper | relative | N- Z- C- I- D- V- | 2** */] = []test{
		{
			func() { F(flagN) },
			"BPL", []byte{0x10, 0x10}, 2,
			func() { EQ(0x02, cpu.PCL()) },
		}, {
			func() {},
			"BPL", []byte{0x10, 0x10}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x04, cpu.PCH()) },
		}, {
			func() {},
			"BPL", []byte{0x10, 0xE0}, 4,
			func() { EQ(0xE2, cpu.PCL()); EQ(0x03, cpu.PCH()) },
		},
	}
	tests[0x30 /* BMI oper | relative | N- Z- C- I- D- V- | 2** */] = []test{
		{
			func() {},
			"BMI", []byte{0x30, 0x10}, 2,
			func() { EQ(0x02, cpu.PCL()) },
		}, {
			func() { F(flagN) },
			"BMI", []byte{0x30, 0x10}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x04, cpu.PCH()) },
		}, {
			func() { F(flagN) },
			"BMI", []byte{0x30, 0xE0}, 4,
			func() { EQ(0xE2, cpu.PCL()); EQ(0x03, cpu.PCH()) },
		},
	}
	tests[0x50 /* BVC oper | relative | N- Z- C- I- D- V- | 2** */] = []test{
		{
			func() { F(flagV) },
			"BVC", []byte{0x50, 0x10}, 2,
			func() { EQ(0x02, cpu.PCL()) },
		}, {
			func() {},
			"BVC", []byte{0x50, 0x10}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x04, cpu.PCH()) },
		}, {
			func() {},
			"BVC", []byte{0x50, 0xE0}, 4,
			func() { EQ(0xE2, cpu.PCL()); EQ(0x03, cpu.PCH()) },
		},
	}
	tests[0x70 /* BVS oper | relative | N- Z- C- I- D- V- | 2** */] = []test{
		{
			func() {},
			"BVS", []byte{0x70, 0x10}, 2,
			func() { EQ(0x02, cpu.PCL()) },
		}, {
			func() { F(flagV) },
			"BVS", []byte{0x70, 0x10}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x04, cpu.PCH()) },
		}, {
			func() { F(flagV) },
			"BVS", []byte{0x70, 0xE0}, 4,
			func() { EQ(0xE2, cpu.PCL()); EQ(0x03, cpu.PCH()) },
		},
	}
	tests[0x90 /* BCC oper | relative | N- Z- C- I- D- V- | 2** */] = []test{
		{
			func() { F(flagC) },
			"BCC", []byte{0x90, 0x10}, 2,
			func() { EQ(0x02, cpu.PCL()) },
		}, {
			func() {},
			"BCC", []byte{0x90, 0x10}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x04, cpu.PCH()) },
		}, {
			func() {},
			"BCC", []byte{0x90, 0xE0}, 4,
			func() { EQ(0xE2, cpu.PCL()); EQ(0x03, cpu.PCH()) },
		},
	}
	tests[0xB0 /* BCS oper | relative | N- Z- C- I- D- V- | 2** */] = []test{
		{
			func() {},
			"BCS", []byte{0xB0, 0x10}, 2,
			func() { EQ(0x02, cpu.PCL()) },
		}, {
			func() { F(flagC) },
			"BCS", []byte{0xB0, 0x10}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x04, cpu.PCH()) },
		}, {
			func() { F(flagC) },
			"BCS", []byte{0xB0, 0xE0}, 4,
			func() { EQ(0xE2, cpu.PCL()); EQ(0x03, cpu.PCH()) },
		},
	}
	tests[0xD0 /* BNE oper | relative | N- Z- C- I- D- V- | 2** */] = []test{
		{
			func() { F(flagZ) },
			"BNE", []byte{0xD0, 0x10}, 2,
			func() { EQ(0x02, cpu.PCL()) },
		}, {
			func() {},
			"BNE", []byte{0xD0, 0x10}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x04, cpu.PCH()) },
		}, {
			func() {},
			"BNE", []byte{0xD0, 0xE0}, 4,
			func() { EQ(0xE2, cpu.PCL()); EQ(0x03, cpu.PCH()) },
		},
	}
	tests[0xF0 /* BEQ oper | relative | N- Z- C- I- D- V- | 2** */] = []test{
		{
			func() {},
			"BEQ", []byte{0xF0, 0x10}, 2,
			func() { EQ(0x02, cpu.PCL()) },
		}, {
			func() { F(flagZ) },
			"BEQ", []byte{0xF0, 0x10}, 3,
			func() { EQ(0x12, cpu.PCL()); EQ(0x04, cpu.PCH()) },
		}, {
			func() { F(flagZ) },
			"BEQ", []byte{0xF0, 0xE0}, 4,
			func() { EQ(0xE2, cpu.PCL()); EQ(0x03, cpu.PCH()) },
		},
	}

	// ---

	tests[0x11 /* ORA (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- | 5*  */] = []test{
		{
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0xAA); A(0x0F); Y(0x01) },
			"ORA", []byte{0x11, 0x80}, 5,
			func() { EQ(0xAF, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0x00, 0x00, 0xAA); A(0x0F); Y(0x02) },
			"ORA", []byte{0x11, 0x80}, 6,
			func() { EQ(0xAF, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x31 /* AND (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- | 5* */] = []test{
		{
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0xAA); A(0x0F); Y(0x01) },
			"AND", []byte{0x31, 0x80}, 5,
			func() { EQ(0x0A, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0x00, 0x00, 0xAA); A(0x0F); Y(0x02) },
			"AND", []byte{0x31, 0x80}, 6,
			func() { EQ(0x0A, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}
	tests[0x51 /* EOR (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- | 5* */] = []test{
		{
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0xAA); A(0x0F); Y(0x01) },
			"EOR", []byte{0x51, 0x80}, 5,
			func() { EQ(0xA5, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0x00, 0x00, 0xAA); A(0x0F); Y(0x02) },
			"EOR", []byte{0x51, 0x80}, 6,
			func() { EQ(0xA5, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x71 /* ADC (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V+ | 5* */] = []test{
		{
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x80); Y(0x01) },
			"ADC", []byte{0x71, 0x80}, 5,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x80); Y(0x01); F(flagC) },
			"ADC", []byte{0x71, 0x80}, 5,
			func() { EQ(0x01, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x80); Y(0x01); F(flagC) },
			"ADC", []byte{0x71, 0x80}, 5,
			func() { EQ(0x01, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x90); Y(0x01); F(flagD) },
			"ADC", []byte{0x71, 0x80}, 5,
			func() { EQ(0x70, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0x00, 0x00, 0x80); A(0x90); Y(0x02); F(flagD | flagC) },
			"ADC", []byte{0x71, 0x80}, 6,
			func() { EQ(0x71, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}
	tests[0x91 /* STA (oper),Y | (indirect),Y | N- Z- C- I- D- V- | 6  */] = []test{
		{
			func() { W(0x80, 0x00, 0xFE, 0xFF); A(0xAA); Y(0x01) },
			"STA", []byte{0x91, 0x80}, 6,
			func() { EQ(0xAA, R(0xFF, 0xFF)) },
		},
	}
	tests[0xB1 /* LDA (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- | 5* */] = []test{
		{
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0xAA); Y(0x01) },
			"LDA", []byte{0xB1, 0x80}, 5,
			func() { EQ(0xAA, cpu.a); EX(H(flagN)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0x00, 0x00, 0xAA); Y(0x02) },
			"LDA", []byte{0xB1, 0x80}, 6,
			func() { EQ(0xAA, cpu.a); EX(H(flagN)) },
		},
	}
	tests[0xD1 /* CMP (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V- | 5* */] = []test{
		{
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x80); Y(0x01) },
			"CMP", []byte{0xD1, 0x80}, 5,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x81); Y(0x01) },
			"CMP", []byte{0xD1, 0x80}, 5,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); A(0x81); Y(0x81) },
			"CMP", []byte{0xD1, 0x80}, 6,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0x00, 0x00, 0x80); A(0x01); Y(0x02) },
			"CMP", []byte{0xD1, 0x80}, 6,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0x00, 0x00, 0x88); A(0x01); Y(0x02) },
			"CMP", []byte{0xD1, 0x80}, 6,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xF1 /* SBC (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V+ | 5* */] = []test{
		{
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x80); Y(0x01) },
			"SBC", []byte{0xF1, 0x80}, 5,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x80); Y(0x01); F(flagC) },
			"SBC", []byte{0xF1, 0x80}, 5,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0x00, 0x00, 0x80); A(0x80); Y(0x02) },
			"SBC", []byte{0xF1, 0x80}, 6,
			func() { EQ(0xFF, cpu.a) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x90); Y(0x01); F(flagD) },
			"SBC", []byte{0xF1, 0x80}, 5,
			func() { EQ(0x09, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xFE, 0xFF); W(0xFF, 0xFF, 0x80); A(0x90); Y(0x01); F(flagC | flagD) },
			"SBC", []byte{0xF1, 0x80}, 5,
			func() { EQ(0x10, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}

	// ---

	tests[0x12 /* HLT */] = []test{{func() {}, "HLT", []byte{0x12}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0x32 /* HLT */] = []test{{func() {}, "HLT", []byte{0x32}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0x52 /* HLT */] = []test{{func() {}, "HLT", []byte{0x52}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0x72 /* HLT */] = []test{{func() {}, "HLT", []byte{0x72}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0x92 /* HLT */] = []test{{func() {}, "HLT", []byte{0x92}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0xB2 /* HLT */] = []test{{func() {}, "HLT", []byte{0xB2}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0xD2 /* HLT */] = []test{{func() {}, "HLT", []byte{0xD2}, 0, func() { EX(cpu.error == ErrHalted) }}}
	tests[0xF2 /* HLT */] = []test{{func() {}, "HLT", []byte{0xF2}, 0, func() { EX(cpu.error == ErrHalted) }}}

	// ---

	tests[0x14 /* NOP | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() {}, "NOP", []byte{0x14, 0x00}, 4, func() {},
		},
	}
	tests[0x34 /* NOP | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() {}, "NOP", []byte{0x34, 0x00}, 4, func() {},
		},
	}
	tests[0x54 /* NOP | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() {}, "NOP", []byte{0x54, 0x00}, 4, func() {},
		},
	}
	tests[0x74 /* NOP | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() {}, "NOP", []byte{0x74, 0x00}, 4, func() {},
		},
	}
	tests[0x94 /* STY oper,X | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() { X(0x70); Y(0x80) },
			"STY", []byte{0x94, 0x10}, 4,
			func() { EQ(0x80, R(0x80, 0x00)) },
		},
	}
	tests[0xB4 /* LDY oper,X | zeropage,X | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); X(0x70) },
			"LDY", []byte{0xB4, 0x10}, 4,
			func() { EQ(0x80, cpu.y) },
		},
	}
	tests[0xD4 /* NOP | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() {}, "NOP", []byte{0xD4, 0x00}, 4, func() {},
		},
	}
	tests[0xF4 /* NOP | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() {}, "NOP", []byte{0xF4, 0x00}, 4, func() {},
		},
	}

	// ---

	tests[0x15 /* ORA oper,X | zeropage,X | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); A(0x01); X(0x70) },
			"ORA", []byte{0x15, 0x10}, 4,
			func() { EQ(0x81, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x35 /* AND oper,X | zeropage,X | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x80, 0x00, 0x0A); A(0xFF); X(0x70) },
			"AND", []byte{0x35, 0x10}, 4,
			func() { EQ(0x0A, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}
	tests[0x55 /* EOR oper,X | zeropage,X | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x80, 0x00, 0xAA); A(0xFF); X(0x70) },
			"EOR", []byte{0x55, 0x10}, 4,
			func() { EQ(0x55, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}
	tests[0x75 /* ADC oper,X | zeropage,X | N+ Z+ C+ I- D- V+ | 4  */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); A(0x80); X(0x70) },
			"ADC", []byte{0x75, 0x10}, 4,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x80); X(0x70); F(flagC) },
			"ADC", []byte{0x75, 0x10}, 4,
			func() { EQ(0x01, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x90); X(0x70); F(flagD) },
			"ADC", []byte{0x75, 0x10}, 4,
			func() { EQ(0x70, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x90); X(0x70); F(flagC | flagD) },
			"ADC", []byte{0x75, 0x10}, 4,
			func() { EQ(0x71, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}
	tests[0x95 /* STA oper,X | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() { X(0x70); A(0x80) },
			"STA", []byte{0x95, 0x10}, 4,
			func() { EQ(0x80, R(0x80, 0x00)) },
		},
	}
	tests[0xB5 /* LDA oper,X | zeropage,X | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); X(0x70) },
			"LDA", []byte{0xB5, 0x10}, 4,
			func() { EQ(0x80, cpu.a) },
		},
	}

	tests[0xD5 /* CMP oper,X | zeropage,X | N+ Z+ C+ I- D- V- | 4 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); A(0x80); X(0x70) },
			"CMP", []byte{0xD5, 0x10}, 4,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x81); X(0x70) },
			"CMP", []byte{0xD5, 0x10}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x01); A(0x81); X(0x70) },
			"CMP", []byte{0xD5, 0x10}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x01); X(0x70) },
			"CMP", []byte{0xD5, 0x10}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x88); A(0x01); X(0x70) },
			"CMP", []byte{0xD5, 0x10}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xF5 /* SBC oper,X | zeropage,X | N+ Z+ C+ I- D- V+ | 4 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); A(0x80); X(0x70) },
			"SBC", []byte{0xF5, 0x10}, 4,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x80); X(0x70); F(flagC) },
			"SBC", []byte{0xF5, 0x10}, 4,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x90); X(0x70); F(flagD) },
			"SBC", []byte{0xF5, 0x10}, 4,
			func() { EQ(0x09, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0x80); A(0x90); X(0x70); F(flagC | flagD) },
			"SBC", []byte{0xF5, 0x10}, 4,
			func() { EQ(0x10, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}

	// ---

	tests[0x16 /* ASL oper,X | zeropage,X | N+ Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x80, 0x00, 0x55); X(0x70) },
			"ASL", []byte{0x16, 0x10}, 6,
			func() { EQ(0xAA, R(0x80, 0x00)); EX(H(flagN)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xAA); X(0x70) },
			"ASL", []byte{0x16, 0x10}, 6,
			func() { EQ(0x54, R(0x80, 0x00)); EX(!H(flagN)); EX(H(flagC)) },
		},
	}
	tests[0x36 /* ROL oper,X | zeropage,X | N+ Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x80, 0x00, 0x55); X(0x70) },
			"ROL", []byte{0x36, 0x10}, 6,
			func() { EQ(0xAA, R(0x80, 0x00)); EX(H(flagN)); EX(!H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xAA); F(flagC); X(0x70) },
			"ROL", []byte{0x36, 0x10}, 6,
			func() { EQ(0x55, R(0x80, 0x00)); EX(!H(flagN)); EX(H(flagC)) },
		},
	}
	tests[0x56 /* LSR oper,X | zeropage,X | N0 Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x80, 0x00, 0x55); X(0x70) },
			"LSR", []byte{0x56, 0x10}, 6,
			func() { EQ(0x2A, R(0x80, 0x00)); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xAA); X(0x70) },
			"LSR", []byte{0x56, 0x10}, 6,
			func() { EQ(0x55, R(0x80, 0x00)); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x76 /* ROR oper,X | zeropage,X | N+ Z+ C+ I- D- V- | 6 */] = []test{
		{
			func() { W(0x80, 0x00, 0x55); X(0x70) },
			"ROR", []byte{0x76, 0x10}, 6,
			func() { EQ(0x2A, R(0x80, 0x00)); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { W(0x80, 0x00, 0xAA); X(0x70) },
			"ROR", []byte{0x76, 0x10}, 6,
			func() { EQ(0x55, R(0x80, 0x00)); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x96 /* STX oper,Y | zeropage,X | N- Z- C- I- D- V- | 4 */] = []test{
		{
			func() { X(0x80); Y(0x70) },
			"STX", []byte{0x96, 0x10}, 4,
			func() { EQ(0x80, R(0x80, 0x00)) },
		},
	}
	tests[0xB6 /* LDX oper,Y | zeropage,X | N+ Z+ C- I- D- V- | 4 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); Y(0x70) },
			"LDX", []byte{0xB6, 0x10}, 4,
			func() { EQ(0x80, cpu.x) },
		},
	}
	tests[0xD6 /* DEC oper,X | zeropage,X | N+ Z+ C- I- D- V- | 6 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); X(0x70) },
			"DEC", []byte{0xD6, 0x10}, 6,
			func() { EQ(0x7F, R(0x80, 0x00)); EX(!H(flagN)) },
		},
	}
	tests[0xF6 /* INC oper,X | zeropage,X | N+ Z+ C- I- D- V- | 6 */] = []test{
		{
			func() { W(0x80, 0x00, 0x80); X(0x70) },
			"INC", []byte{0xF6, 0x10}, 6,
			func() { EQ(0x81, R(0x80, 0x00)); EX(H(flagN)) },
		},
	}

	// ---

	tests[0x18 /* CLC | implied | N- Z- C0 I- D- V- | 2 */] = []test{
		{
			func() { F(flagC) },
			"CLC", []byte{0x18}, 2,
			func() { EX(!H(flagC)) },
		},
	}
	tests[0x38 /* SEC | implied | N- Z- C1 I- D- V- | 2 */] = []test{
		{
			func() { cpu.p.set(false, flagC) },
			"SEC", []byte{0x38}, 2,
			func() { EX(H(flagC)) },
		},
	}
	tests[0x58 /* CLI | implied | N- Z- C- I0 D- V- | 2 */] = []test{
		{
			func() { F(flagI) },
			"CLI", []byte{0x58}, 2,
			func() { EX(!cpu.p.has(flagI)) },
		},
	}
	tests[0x78 /* SEI | implied | N- Z- C- I1 D- V- | 2 */] = []test{
		{
			func() { cpu.p.set(false, flagI) },
			"SEI", []byte{0x78}, 2,
			func() { EX(cpu.p.has(flagI)) },
		},
	}
	tests[0x98 /* TYA | implied | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { Y(0x80) },
			"TYA", []byte{0x98}, 2,
			func() { EQ(0x80, cpu.a); EX(H(flagN)) },
		},
	}
	tests[0xB8 /* CLV | implied | N- Z- C- I- D- V0 | 2  */] = []test{
		{
			func() { F(flagV) },
			"CLV", []byte{0xB8}, 2,
			func() { EX(!cpu.p.has(flagV)) },
		},
	}
	tests[0xD8 /* CLD | implied | N- Z- C- I- D0 V- | 2 */] = []test{
		{
			func() { F(flagD) },
			"CLD", []byte{0xD8}, 2,
			func() { EX(!cpu.p.has(flagD)) },
		},
	}
	tests[0xF8 /* SED | implied | N- Z- C- I- D1 V- | 2 */] = []test{
		{
			func() {},
			"SED", []byte{0xF8}, 2,
			func() { EX(cpu.p.has(flagD)) },
		},
	}

	// ---

	tests[0x19 /* ORA oper,Y | absolute,Y | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); Y(0x02); A(0x01) },
			"ORA", []byte{0x19, 0x10, 0x34}, 4,
			func() { EQ(0x81, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		}, {
			func() { Y(0x02); A(0x01) },
			"ORA", []byte{0x19, 0xFF, 0xFF}, 5,
			func() { EQ(0x01, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}
	tests[0x39 /* AND oper,Y | absolute,Y | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0xAA); Y(0x02); A(0xFF) },
			"AND", []byte{0x39, 0x10, 0x34}, 4,
			func() { EQ(0xAA, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		}, {
			func() { Y(0x02); A(0xFF) },
			"AND", []byte{0x39, 0xFF, 0xFF}, 5,
			func() { EQ(0x00, cpu.a) },
		},
	}
	tests[0x59 /* EOR oper,Y | absolute,Y | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0xAA); Y(0x02); A(0xFF) },
			"EOR", []byte{0x59, 0x10, 0x34}, 4,
			func() { EQ(0x55, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		}, {
			func() { Y(0x02); A(0xFF) },
			"EOR", []byte{0x59, 0xFF, 0xFF}, 5,
			func() { EQ(0xFF, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x79 /* ADC oper,Y | absolute,Y | N+ Z+ C+ I- D- V+ | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); Y(0x02); A(0x80) },
			"ADC", []byte{0x79, 0x10, 0x34}, 4,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); Y(0x02); A(0x80); F(flagC) },
			"ADC", []byte{0x79, 0x10, 0x34}, 4,
			func() { EQ(0x01, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); Y(0x02); A(0x90); F(flagD) },
			"ADC", []byte{0x79, 0x10, 0x34}, 4,
			func() { EQ(0x70, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x00, 0x00, 0x80); Y(0x02); A(0x90); F(flagC | flagD) },
			"ADC", []byte{0x79, 0xFE, 0xFF}, 5,
			func() { EQ(0x71, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}
	tests[0x99 /* STA oper,Y | absolute,Y | N- Z- C- I- D- V- | 5 */] = []test{
		{
			func() { A(0x80); Y(0xFF) },
			"STA", []byte{0x99, 0x12, 0x34}, 5,
			func() { EQ(0x80, R(0x11, 0x35)) },
		},
	}
	tests[0xB9 /* LDA oper,Y | absolute,Y | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); Y(0x01) },
			"LDA", []byte{0xB9, 0x11, 0x34}, 4,
			func() { EQ(0x80, cpu.a); EX(H(flagN)) },
		}, {
			func() { W(0x11, 0x35, 0x80); Y(0xFF) },
			"LDA", []byte{0xB9, 0x12, 0x34}, 5,
			func() { EQ(0x80, cpu.a); EX(!H(flagZ)) },
		},
	}
	tests[0xD9 /* CMP oper,Y | absolute,Y | N+ Z+ C+ I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); A(0x80); Y(0x01) },
			"CMP", []byte{0xD9, 0x11, 0x34}, 4,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x81); Y(0x01) },
			"CMP", []byte{0xD9, 0x11, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x01); A(0x81); Y(0x01) },
			"CMP", []byte{0xD9, 0x11, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x01); Y(0x01) },
			"CMP", []byte{0xD9, 0x11, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x88); A(0x01); Y(0x01) },
			"CMP", []byte{0xD9, 0x11, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}
	tests[0xF9 /* SBC oper,Y | absolute,Y | N+ Z+ C+ I- D- V+ | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); A(0x80); Y(0x01) },
			"SBC", []byte{0xF9, 0x11, 0x34}, 4,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x80); Y(0x01); F(flagC) },
			"SBC", []byte{0xF9, 0x11, 0x34}, 4,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x00, 0x00, 0x80); A(0x80); Y(0x01) },
			"SBC", []byte{0xF9, 0xFF, 0xFF}, 5,
			func() { EQ(0xFF, cpu.a) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x90); Y(0x01); F(flagD) },
			"SBC", []byte{0xF9, 0x11, 0x34}, 4,
			func() { EQ(0x09, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x90); Y(0x01); F(flagC | flagD) },
			"SBC", []byte{0xF9, 0x11, 0x34}, 4,
			func() { EQ(0x10, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}

	// ---

	tests[0x1A /* NOP | implied | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0x1A}, 2, func() {},
		},
	}
	tests[0x3A /* NOP | implied | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0x3A}, 2, func() {},
		},
	}
	tests[0x5A /* NOP | implied | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0x5A}, 2, func() {},
		},
	}
	tests[0x7A /* NOP | implied | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0x7A}, 2, func() {},
		},
	}
	tests[0x9A /* TXS | implied | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() { X(0x80) },
			"TXS", []byte{0x9A}, 2,
			func() { EQ(0x80, cpu.s) },
		},
	}
	tests[0xBA /* TSX | implied | N+ Z+ C- I- D- V- | 2 */] = []test{
		{
			func() { cpu.s = 0x80 },
			"TSX", []byte{0xBA}, 2,
			func() { EQ(0x80, cpu.x); EX(H(flagN)) },
		},
	}
	tests[0xDA /* NOP | implied | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0xDA}, 2, func() {},
		},
	}
	tests[0xFA /* NOP | implied | N- Z- C- I- D- V- | 2 */] = []test{
		{
			func() {}, "NOP", []byte{0xFA}, 2, func() {},
		},
	}

	// ---

	tests[0x1C /* NOP | absolute,X | N- Z- C- I- D- V- | 4* */] = []test{
		{
			func() {}, "NOP", []byte{0x1C}, 4, func() {},
		},
	}
	tests[0x3C /* NOP | absolute,X | N- Z- C- I- D- V- | 4* */] = []test{
		{
			func() {}, "NOP", []byte{0x3C}, 4, func() {},
		},
	}
	tests[0x5C /* NOP | absolute,X | N- Z- C- I- D- V- | 4* */] = []test{
		{
			func() {}, "NOP", []byte{0x5C}, 4, func() {},
		},
	}
	tests[0x7C /* NOP | absolute,X | N- Z- C- I- D- V- | 4* */] = []test{
		{
			func() {}, "NOP", []byte{0x7C}, 4, func() {},
		},
	}
	tests[0x9C /* invalid */] = nil
	tests[0xBC /* LDY oper,X | absolute,X | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); cpu.x = 0x1 },
			"LDY", []byte{0xBC, 0x11, 0x34}, 4,
			func() { EQ(0x80, cpu.y); EX(H(flagN)) },
		}, {
			func() { W(0x00, 0x00, 0x80); cpu.x = 0x1 },
			"LDY", []byte{0xBC, 0xFF, 0xFF}, 5,
			func() { EQ(0x80, cpu.y); EX(H(flagN)) },
		},
	}
	tests[0xDC /* NOP | absolute,X | N- Z- C- I- D- V- | 4* */] = []test{
		{
			func() {}, "NOP", []byte{0xDC}, 4, func() {},
		},
	}
	tests[0xFC /* NOP | absolute,X | N- Z- C- I- D- V- | 4* */] = []test{
		{
			func() {}, "NOP", []byte{0xFC}, 4, func() {},
		},
	}

	// ---

	tests[0x1D /* ORA oper,X | absolute,X | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); X(0x02); A(0x01) },
			"ORA", []byte{0x1D, 0x10, 0x34}, 4,
			func() { EQ(0x81, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		}, {
			func() { X(0x02); A(0x01) },
			"ORA", []byte{0x1D, 0xFF, 0xFF}, 5,
			func() { EQ(0x01, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		},
	}
	tests[0x3D /* AND oper,X | absolute,X | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0xAA); X(0x02); A(0xFF) },
			"AND", []byte{0x3D, 0x10, 0x34}, 4,
			func() { EQ(0xAA, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		}, {
			func() { X(0x02); A(0xFF) },
			"AND", []byte{0x3D, 0xFF, 0xFF}, 5,
			func() { EQ(0x00, cpu.a) },
		},
	}
	tests[0x5D /* EOR oper,X | absolute,X | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0xAA); X(0x02); A(0xFF) },
			"EOR", []byte{0x5D, 0x10, 0x34}, 4,
			func() { EQ(0x55, cpu.a); EX(!H(flagZ)); EX(!H(flagN)) },
		}, {
			func() { X(0x02); A(0xFF) },
			"EOR", []byte{0x5D, 0xFF, 0xFF}, 5,
			func() { EQ(0xFF, cpu.a); EX(!H(flagZ)); EX(H(flagN)) },
		},
	}
	tests[0x7D /* ADC oper,X | absolute,X | N+ Z+ C+ I- D- V+ | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); X(0x02); A(0x80) },
			"ADC", []byte{0x7D, 0x10, 0x34}, 4,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); X(0x02); A(0x80); F(flagC) },
			"ADC", []byte{0x7D, 0x10, 0x34}, 4,
			func() { EQ(0x01, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); X(0x02); A(0x90); F(flagD) },
			"ADC", []byte{0x7D, 0x10, 0x34}, 4,
			func() { EQ(0x70, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x00, 0x00, 0x80); X(0x02); A(0x90); F(flagC | flagD) },
			"ADC", []byte{0x7D, 0xFE, 0xFF}, 5,
			func() { EQ(0x71, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}
	tests[0x9D /* STA oper,X | absolute,X | N- Z- C- I- D- V- | 5 */] = []test{
		{
			func() { A(0x80) },
			"STA", []byte{0x9D, 0x12, 0x34}, 5,
			func() { EQ(0x80, R(0x12, 0x34)) },
		},
	}
	tests[0xBD /* LDA oper,X | absolute,X | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); X(0x01) },
			"LDA", []byte{0xBD, 0x11, 0x34}, 4,
			func() { EQ(0x80, cpu.a); EX(H(flagN)) },
		}, {
			func() { W(0x11, 0x35, 0x80); X(0xFF) },
			"LDA", []byte{0xBD, 0x12, 0x34}, 5,
			func() { EQ(0x80, cpu.a); EX(!H(flagZ)) },
		},
	}
	tests[0xDD /* CMP oper,X | absolute,X | N+ Z+ C+ I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); A(0x80); X(0x01) },
			"CMP", []byte{0xDD, 0x11, 0x34}, 4,
			func() { EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x81); X(0x01) },
			"CMP", []byte{0xDD, 0x11, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x01); A(0x81); X(0x01) },
			"CMP", []byte{0xDD, 0x11, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x01); X(0x01) },
			"CMP", []byte{0xDD, 0x11, 0x34}, 4,
			func() { EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x88); A(0x01); X(0x01) },
			"CMP", []byte{0xDD, 0x11, 0x34}, 4,
			func() { EX(!H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		},
	}

	tests[0xFD /* SBC oper,X | absolute,X | N+ Z+ C+ I- D- V+ | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); A(0x80); X(0x01) },
			"SBC", []byte{0xFD, 0x11, 0x34}, 4,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x80); X(0x01); F(flagC) },
			"SBC", []byte{0xFD, 0x11, 0x34}, 4,
			func() { EQ(0x00, cpu.a); EX(!H(flagN)); EX(H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x00, 0x00, 0x80); A(0x80); X(0x01) },
			"SBC", []byte{0xFD, 0xFF, 0xFF}, 5,
			func() { EQ(0xFF, cpu.a); EX(H(flagN)); EX(!H(flagZ)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x90); X(0x01); F(flagD) },
			"SBC", []byte{0xFD, 0x11, 0x34}, 4,
			func() { EQ(0x09, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0x80); A(0x90); X(0x01); F(flagC | flagD) },
			"SBC", []byte{0xFD, 0x11, 0x34}, 4,
			func() { EQ(0x10, cpu.a); EX(!H(flagN)); EX(!H(flagZ)); EX(H(flagC)) },
		},
	}

	// ---

	tests[0x1E /* ASL oper,X | absolute,X | N+ Z+ C+ I- D- V- | 7 */] = []test{
		{
			func() { W(0x12, 0x34, 0x55); X(0x01) },
			"ASL", []byte{0x1E, 0x11, 0x34}, 7,
			func() { EQ(0xAA, R(0x12, 0x34)); EX(H(flagN)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0xAA); X(0x01) },
			"ASL", []byte{0x1E, 0x11, 0x34}, 7,
			func() { EQ(0x54, R(0x12, 0x34)); EX(!H(flagN)); EX(H(flagC)) },
		},
	}
	tests[0x3E /* ROL oper,X | absolute,X | N+ Z+ C+ I- D- V- | 7 */] = []test{
		{
			func() { W(0x12, 0x34, 0x55); X(0x01) },
			"ROL", []byte{0x3E, 0x11, 0x34}, 7,
			func() { EQ(0xAA, R(0x12, 0x34)); EX(H(flagN)); EX(!H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0xAA); F(flagC); X(0x01) },
			"ROL", []byte{0x3E, 0x11, 0x34}, 7,
			func() { EQ(0x55, R(0x12, 0x34)); EX(!H(flagN)); EX(H(flagC)) },
		},
	}
	tests[0x5E /* LSR oper,X | absolute,X | N0 Z+ C+ I- D- V- | 7 */] = []test{
		{
			func() { W(0x12, 0x34, 0x55); X(0x01) },
			"LSR", []byte{0x5E, 0x11, 0x34}, 7,
			func() { EQ(0x2A, R(0x12, 0x34)); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0xAA); X(0x01) },
			"LSR", []byte{0x5E, 0x11, 0x34}, 7,
			func() { EQ(0x55, R(0x12, 0x34)); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x7E /* ROR oper,X | absolute,X | N+ Z+ C+ I- D- V- | 7 */] = []test{
		{
			func() { W(0x12, 0x34, 0x55); X(0x01) },
			"ROR", []byte{0x7E, 0x11, 0x34}, 7,
			func() { EQ(0x2A, R(0x12, 0x34)); EX(!H(flagN)); EX(H(flagC)) },
		}, {
			func() { W(0x12, 0x34, 0xAA); X(0x01) },
			"ROR", []byte{0x7E, 0x11, 0x34}, 7,
			func() { EQ(0x55, R(0x12, 0x34)); EX(!H(flagN)); EX(!H(flagC)) },
		},
	}
	tests[0x9E /* invalid */] = nil
	tests[0xBE /* LDX oper,Y | absolute,Y | N+ Z+ C- I- D- V- | 4* */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); Y(0x01) },
			"LDX", []byte{0xBE, 0x11, 0x34}, 4,
			func() { EQ(0x80, cpu.x); EX(H(flagN)); EX(!H(flagZ)) },
		}, {
			func() { W(0x00, 0x00, 0x80); Y(0x01) },
			"LDX", []byte{0xBE, 0xFF, 0xFF}, 5,
			func() { EQ(0x80, cpu.x); EX(H(flagN)); EX(!H(flagZ)) },
		},
	}
	tests[0xDE /* DEC oper,X | absolute,X | N+ Z+ C- I- D- V- | 7 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); X(0x01) },
			"DEC", []byte{0xDE, 0x11, 0x34}, 7,
			func() { EQ(0x7F, R(0x12, 0x34)); EX(!H(flagN)) },
		},
	}
	tests[0xFE /* INC oper,X | absolute,X | N+ Z+ C- I- D- V- | 7 */] = []test{
		{
			func() { W(0x12, 0x34, 0x80); X(0x01) },
			"INC", []byte{0xFE, 0x11, 0x34}, 7,
			func() { EQ(0x81, R(0x12, 0x34)); EX(H(flagN)) },
		},
	}

	for i := range tests {
		if tests[i] == nil {
			continue
		}
		for _, tt := range tests[i] {

			bus.Reset()
			for k, b := range tt.mem {
				bus.mem[k+0x0400] = b
			}

			cpu.Reset()
			cpu.PC(0x00, 0x04)

			tt.init()

			cost, err := cpu.Step()
			if err != nil && !errors.Is(err, ErrHalted) {
				t.Error(err)
			}

			EQ(byte(tt.cost), byte(cost))
			//t.Logf("0x%02X %s", tt.mem[0], tt.mne)

			tt.post()
		}
	}
}

func TestFlag(t *testing.T) {
	f := 0xFF ^ flagD
	if s := (&f).String(); s != "NV-IZC" {
		t.Fatalf("unexpected, got %s", s)
	}
}

func TestHalt(t *testing.T) {
	bus := &memoryBus{}
	bus.mem[0x00] = 0x02
	cpu := New(bus)

	_, err := cpu.Step()
	if err == nil {
		t.Fatal("unexpected")
	}
	if !errors.Is(err, ErrHalted) {
		t.Logf("unexpected, got %s", err)
	}
	_, err = cpu.Step()
	if !errors.Is(err, ErrHalted) {
		t.Logf("unexpected, got %s", err)
	}
}

func TestInvalid(t *testing.T) {
	bus := &memoryBus{}
	bus.mem[0x00] = 0x9E
	cpu := New(bus)

	_, err := cpu.Step()
	if err == nil {
		t.Fatal("unexpected")
	}
	if "m6502: invalid op code: 0000: 9E" != err.Error() {
		t.Logf("unexpected, got '%s'", err)
	}
}

func TestNMI(t *testing.T) {
	bus := &memoryBus{}
	bus.mem[0xFFFA] = 0x12
	bus.mem[0xFFFB] = 0x34

	cpu := New(bus)
	cpu.NMI()

	if cpu.PCL() != 0x12 || cpu.PCH() != 0x34 || cpu.s != 0xFC {
		t.Log("unexpected")
	}
}

func TestIRQ(t *testing.T) {
	bus := &memoryBus{}
	bus.mem[0xFFFE] = 0x12
	bus.mem[0xFFFF] = 0x34

	cpu := New(bus)

	cpu.p.set(true, flagI)
	cpu.IRQ()
	if cpu.PCL() != 0x00 || cpu.PCH() != 0x00 || cpu.s != 0xFF {
		t.Log("unexpected")
	}

	cpu.p.set(false, flagI)
	cpu.IRQ()
	if cpu.PCL() != 0x12 || cpu.PCH() != 0x34 || cpu.s != 0xFC {
		t.Log("unexpected")
	}
}

func TestString(t *testing.T) {
	cpu := New(&memoryBus{})
	if "m6502: PC=0000 A=00 X=00 Y=00 [------] S=FF" != cpu.String() {
		t.Logf("unexpected, got %s", cpu.String())
	}
}

type panicBus struct{ mem [0x10000 - 2]byte }

func (*panicBus) Read(l, _ byte) byte {
	if l == 0x00 {
		panic("foo")
	}
	return 0x00
}
func (*panicBus) Write(_, _, _ byte) {}

func TestPanic(t *testing.T) {
	bus := &panicBus{}
	cpu := New(bus)

	_, err := cpu.Step()
	if err == nil {
		t.Fatal("unexpected")
	}
	if "foo" != err.Error() {
		t.Logf("unexpected, got *%s*", err)
	}
}

func BenchmarkCPU(b *testing.B) {
	bus := &memoryBus{}
	cpu := New(bus)

	file, err := os.Open("./dev/6502_functional_test.bin")
	if err != nil {
		b.Fatal(err)
	}
	_, err = io.ReadFull(file, bus.mem[:])
	if err != nil {
		b.Fatal(err)
	}
	cpu.PC(0x00, 0x04)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		for {
			_, err = cpu.Step()
			if err != nil {
				b.Fatal(err)
			}
			if cpu.PCH() == 0x34 && cpu.PCL() == 0x69 {
				break
			}
		}
	}
}
