// MIT License · Daniel T. Gorski · dtg [at] lengo [dot] org · 09/2023

// Package m6502 is a lightweight cycle-accurate MOS 6502 CPU emulator library for Go.
package m6502

import (
	"errors"
	"fmt"
)

type (
	// Bus is a 8-bit data bus with a 16-bit little-endian address width.
	Bus interface {

		// Read reads a byte from address space. With the current CPU
		// implementation, Read is allowed to panic, e.g. when reading
		// from unmapped memory.
		Read(lo, hi byte) byte

		// Write writes a byte to address space. With the current CPU
		// implementation Write is allowed to panic, e.g. when writing
		// to unmapped memory.
		Write(lo, hi, db byte)
	}

	// CPU represents the 6502 emulator.
	CPU struct {
		bus Bus

		a byte  // Accumulator
		x byte  // X register
		y byte  // Y register
		s byte  // Stack pointer
		p *flag // Processor flags

		pcl byte // Program counter low
		pch byte // Program counter high

		cycles uint
		halted bool
	}

	flag byte
)

const (
	flagN flag = 1 << 7 // N | Negative, set if bit 7 set
	flagV flag = 1 << 6 // V | Overflow, sign bit is incorrect
	flagU flag = 1 << 5 // - | Unused
	flagB flag = 1 << 4 // B | Break command (stack only)
	flagD flag = 1 << 3 // D | Decimal mode
	flagI flag = 1 << 2 // I | Interrupt disable
	flagZ flag = 1 << 1 // Z | Zero flag
	flagC flag = 1 << 0 // C | Set if overflow in bit 7
)

var (
	// ErrHalted will be returned from Step() when CPU was halted.
	ErrHalted = fmt.Errorf("CPU halted")
)

// New creates a new 6502 CPU. This method will panic when the Bus does not have access
// to the Reset Vector memory (0xFFFC/FD): When the CPU is created, the program counter
// will be set to the Reset Vector values found at 0xFFFC and 0xFFFD.
func New(bus Bus) *CPU {
	cpu := &CPU{bus: bus}
	cpu.Reset()
	return cpu
}

// PC sets the CPU program counter.
func (cpu *CPU) PC(lo, hi byte) {
	cpu.pcl, cpu.pch = lo, hi
}

// PCL returns the lower byte of the CPU program counter.
func (cpu *CPU) PCL() byte {
	return cpu.pcl
}

// PCH returns the higher byte of the CPU program counter.
func (cpu *CPU) PCH() byte {
	return cpu.pch
}

// NMI processes a non-maskable interrupt.
func (*CPU) NMI() {
}

// IRQ processes an interrupt request.
func (*CPU) IRQ() {
}

// Reset resets the CPU to initial state. The program counter
// is set to value of the default Reset Vector (0xFFFC/FD).
func (cpu *CPU) Reset() {
	cpu.s, cpu.a, cpu.x, cpu.y = 0xFF, 0x00, 0x00, 0x00
	cpu.pcl = cpu.bus.Read(0xFC, 0xFF)
	cpu.pch = cpu.bus.Read(0xFD, 0xFF)
	flg := flag(0)
	cpu.p = &flg
	cpu.halted = false
	cpu.cycles = 0
}

// Step performs *one* instruction and returns the number of cycles, that the original
// processor would have needed. Use this value to control the time penalty regime.
// A panic on the underlying bus read/write will be recovered and converted to an error.
// When the CPU is halted by an instruction, this function will immediately return
// an ErrHalted error until a Reset().
func (cpu *CPU) Step() (cycles uint, err error) {
	if cpu.halted {
		return 0, ErrHalted
	}
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()
	if err = cpu.tick(); err != nil {
		return 0, err
	}
	return cpu.cycles, err
}

func (cpu *CPU) String() string {
	return fmt.Sprintf(
		"[m6502] PC=%02X%02X A=%02X X=%02X Y=%02X [%s] S=%02X",
		cpu.PCH(), cpu.PCL(), cpu.a, cpu.x, cpu.y, cpu.p, cpu.s,
	)
}

func (cpu *CPU) tick() error {
	cpu.cycles = 0
	pcl, pch := cpu.pcl, cpu.pch

	type B = byte
	type C = bool // Read: "condition"
	type F = flag

	when := func(d C, t, g B) B {
		if d {
			return t
		}
		return g
	}
	cost := func(n B) { cpu.cycles += uint(n) }

	uadd := func(a, b B) (B, B) { s := a + b; return s, when(s < b, 0x01, 0x00) }
	ovfl := func(s int16) B { return when(s>>8 > 0x00, 0x01, when(s < 0, 0xFF, 0x00)) }
	sadd := func(a B, b int8) (B, B) { s := int16(a) + int16(b); return B(s), ovfl(s) }
	inc := func(l, h B) (B, B) { l, c := uadd(l, 0x01); return l, h + c }

	setPC := func(l, h B) { cpu.pcl, cpu.pch = l, h }
	incPC := func() { setPC(inc(cpu.pcl, cpu.pch)) }

	read := func(l, h B) B { cost(1); return cpu.bus.Read(l, h) }
	zread := func(l B) B { return read(l, 0x00) }
	vread := func(l B) (B, B) { return read(l, 0xFF), read(l+1, 0xFF) }
	write := func(l, h, b B) { cost(1); cpu.bus.Write(l, h, b) }
	zwrite := func(l, b B) { write(l, 0x00, b) }
	fetch := func() B { b := read(cpu.pcl, cpu.pch); incPC(); return b }

	setF := func(c C, f F) { cpu.p.set(c, f) }
	hasF := func(f F) C { return cpu.p.has(f) }

	setC := func(c C) { setF(c, flagC) }
	setI := func(c C) { setF(c, flagI) }
	setN := func(b B) { setF(b&0x80 != 0x00, flagN) }
	setNZ := func(b B) B { setN(b); setF(b == 0x00, flagZ); return b }

	setA := func(b B) { cpu.a = setNZ(b) }
	setX := func(b B) { cpu.x = setNZ(b) }
	setY := func(b B) { cpu.y = setNZ(b) }

	push := func(b B) { write(cpu.s, 0x01, b); cpu.s-- }
	pop := func() B { cpu.s++; return read(cpu.s, 0x01) }

	pushPC := func() { push(cpu.pch); push(cpu.pcl) }
	popPC := func() (B, B) { return pop(), pop() }

	php := func() { push(B(*cpu.p | flagU | flagB)) }
	plp := func() { *cpu.p = F(pop()) & ^(flagU | flagB) }

	cmp := func(a, b B) { setNZ(b - a); setC(b >= a) }
	bit := func(b B) { setN(b); setF(b&cpu.a == 0, flagZ); setF(b&0x40 != 0, flagV) }

	asl := func(b B) B { setC(b&0x80 != 0); return setNZ(b << 1) }
	lsr := func(b B) B { setC(b&0x01 != 0); return setNZ(b >> 1) }
	rol := func(b B) B { c := B(*cpu.p & flagC); setC(b&0x80 != 0); return setNZ(b<<1 | c) }
	ror := func(b B) B { c := B(*cpu.p & flagC); setC(b&0x01 != 0); return setNZ(b>>1 | c<<7) }

	abs := func() (B, B) { return fetch(), fetch() }
	absN := func(n B) (B, B, B) { l, c := uadd(fetch(), n); return l, fetch() + c, c }
	relN := func(n B) (B, B, B) { l, o := sadd(cpu.pcl, int8(n)); return l, cpu.pch + o, o }

	indY := func() (B, B, B) { b := fetch(); l, c := uadd(zread(b), cpu.y); return l, zread(b+1) + c, c }
	indX := func() (B, B) { b := fetch() + cpu.x; return zread(b), zread(b + 1) }

	adc := func(b B) B {
		if cpu.p.has(flagD) {
			l := cpu.a&0x0F + b&0x0F + when(hasF(flagC), 0x01, 0x00)
			l += when(l&0xFF > 9, 6, 0)
			h := cpu.a>>4 + b>>4 + when(l > 0x0F, 1, 0)
			h += when(h&0xFF > 9, 6, 0)
			setC(h > 0x0F)
			return l&0x0F | (h<<4)&0xF0
		}
		w := uint16(cpu.a) + uint16(b) + uint16(when(hasF(flagC), 0x01, 0x00))
		r := B(w)
		setC(w > 0xFF)
		setF((cpu.a^r)&(b^r)&0x80 != 0x00, flagV)
		return r
	}
	sbc := func(b B) B {
		if cpu.p.has(flagD) {
			l := (cpu.a & 0x0F) - (b & 0x0F) - when(hasF(flagC), 0x00, 0x01)
			l -= when(l&0x10 != 0, 6, 0)
			h := (cpu.a >> 4) - (b >> 4) - when((l&0x10) != 0, 1, 0)
			h -= when(h&0x10 != 0, 6, 0)
			setC(h&0xFF < 0x0F)
			return l&0x0F | h<<4
		}
		return adc(^b)
	}
	branch := func(c C) {
		if b := fetch(); c {
			l, h, o := relN(b)
			cost(1 + when(o == 0, 0, 1))
			setPC(l, h)
		}
	}

	// ---

	//  * add 1 to cycles if page boundary is crossed
	// ** add 1 to cycles if branch occurs on same page
	// ** add 2 to cycles if branch occurs to different page

	type operation [0x100]func()
	op := operation{}

	op[0x00 /* BRK | 7 */ ] = func() { fetch(); pushPC(); php(); setPC(vread(0xFE)); setI(true) }
	op[0x20 /* JSR | 6 */ ] = func() { l := fetch(); pushPC(); setPC(l, fetch()); cost(1) }
	op[0x40 /* RTI | 7 */ ] = func() { plp(); setPC(popPC()); cost(3) }
	op[0x60 /* RTS | 6 */ ] = func() { setPC(inc(popPC())); cost(3) }
	op[0x80 /* NOP | 2 */ ] = func() { cost(1) }
	op[0xA0 /* LDY | 2 */ ] = func() { setY(fetch()) }
	op[0xC0 /* CPY | 2 */ ] = func() { cmp(fetch(), cpu.y) }
	op[0xE0 /* CPX | 2 */ ] = func() { cmp(fetch(), cpu.x) }

	op[0x01 /* ORA | 6 */ ] = func() { setA(cpu.a | read(indX())); cost(1) }
	op[0x21 /* AND | 6 */ ] = func() { setA(cpu.a & read(indX())); cost(1) }
	op[0x41 /* EOR | 6 */ ] = func() { setA(cpu.a ^ read(indX())); cost(1) }
	op[0x61 /* ADC | 6 */ ] = func() { setA(adc(read(indX()))); cost(1) }
	op[0x81 /* STA | 6 */ ] = func() { l, h := indX(); write(l, h, cpu.a); cost(1) }
	op[0xA1 /* LDA | 6 */ ] = func() { setA(read(indX())); cost(1) }
	op[0xC1 /* CMP | 6 */ ] = func() { cmp(read(indX()), cpu.a); cost(1) }
	op[0xE1 /* SBC | 6 */ ] = func() { setA(sbc(read(indX()))); cost(1) }

	op[0x02 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0x22 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0x42 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0x62 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0x82 /* NOP | 2 */ ] = func() { cost(1) }
	op[0xA2 /* LDX | 2 */ ] = func() { setX(fetch()) }
	op[0xC2 /* NOP | 2 */ ] = func() { cost(1) }
	op[0xE2 /* NOP | 2 */ ] = func() { cost(1) }

	op[0x04 /* NOP | 3 */ ] = func() { cost(2) }
	op[0x24 /* BIT | 3 */ ] = func() { bit(zread(fetch())) }
	op[0x44 /* NOP | 3 */ ] = func() { cost(2) }
	op[0x64 /* NOP | 3 */ ] = func() { cost(2) }
	op[0x84 /* STY | 3 */ ] = func() { zwrite(fetch(), cpu.y) }
	op[0xA4 /* LDY | 3 */ ] = func() { setY(zread(fetch())) }
	op[0xC4 /* CPY | 3 */ ] = func() { cmp(zread(fetch()), cpu.y) }
	op[0xE4 /* CPX | 3 */ ] = func() { cmp(zread(fetch()), cpu.x) }

	op[0x05 /* ORA | 3 */ ] = func() { setA(cpu.a | zread(fetch())) }
	op[0x25 /* AND | 3 */ ] = func() { setA(cpu.a & zread(fetch())) }
	op[0x45 /* EOR | 3 */ ] = func() { setA(cpu.a ^ zread(fetch())) }
	op[0x65 /* ADC | 3 */ ] = func() { setA(adc(zread(fetch()))) }
	op[0x85 /* STA | 3 */ ] = func() { zwrite(fetch(), cpu.a) }
	op[0xA5 /* LDA | 3 */ ] = func() { setA(zread(fetch())) }
	op[0xC5 /* CMP | 3 */ ] = func() { cmp(zread(fetch()), cpu.a) }
	op[0xE5 /* SBC | 3 */ ] = func() { setA(sbc(zread(fetch()))) }

	op[0x06 /* ASL | 5 */ ] = func() { b := fetch(); zwrite(b, asl(zread(b))); cost(1) }
	op[0x26 /* ROL | 5 */ ] = func() { b := fetch(); zwrite(b, rol(zread(b))); cost(1) }
	op[0x46 /* LSR | 5 */ ] = func() { b := fetch(); zwrite(b, lsr(zread(b))); cost(1) }
	op[0x66 /* ROR | 5 */ ] = func() { b := fetch(); zwrite(b, ror(zread(b))); cost(1) }
	op[0x86 /* STX | 3 */ ] = func() { zwrite(fetch(), cpu.x) }
	op[0xA6 /* LDX | 3 */ ] = func() { setX(zread(fetch())) }
	op[0xC6 /* DEC | 5 */ ] = func() { b := fetch(); zwrite(b, setNZ(zread(b)-1)); cost(1) }
	op[0xE6 /* INC | 5 */ ] = func() { b := fetch(); zwrite(b, setNZ(zread(b)+1)); cost(1) }

	op[0x08 /* PHP | 3 */ ] = func() { php(); cost(1) }
	op[0x28 /* PLP | 4 */ ] = func() { plp(); cost(2) }
	op[0x48 /* PHA | 3 */ ] = func() { push(cpu.a); cost(1) }
	op[0x68 /* PLA | 4 */ ] = func() { setA(pop()); cost(2) }
	op[0x88 /* DEY | 2 */ ] = func() { setY(cpu.y - 1); cost(1) }
	op[0xA8 /* TAY | 2 */ ] = func() { setY(cpu.a); cost(1) }
	op[0xC8 /* INY | 2 */ ] = func() { setY(cpu.y + 1); cost(1) }
	op[0xE8 /* INX | 2 */ ] = func() { setX(cpu.x + 1); cost(1) }

	op[0x09 /* ORA | 2 */ ] = func() { setA(cpu.a | fetch()) }
	op[0x29 /* AND | 2 */ ] = func() { setA(cpu.a & fetch()) }
	op[0x49 /* EOR | 2 */ ] = func() { setA(cpu.a ^ fetch()) }
	op[0x69 /* ADC | 2 */ ] = func() { setA(adc(fetch())) }
	op[0x89 /* NOP | 2 */ ] = func() { cost(1) }
	op[0xA9 /* LDA | 2 */ ] = func() { setA(fetch()) }
	op[0xC9 /* CMP | 2 */ ] = func() { cmp(fetch(), cpu.a) }
	op[0xE9 /* SBC | 2 */ ] = func() { setA(sbc(fetch())) }

	op[0x0A /* ASL | 2 */ ] = func() { setA(asl(cpu.a)); cost(1) }
	op[0x2A /* ROL | 2 */ ] = func() { setA(rol(cpu.a)); cost(1) }
	op[0x4A /* LSR | 2 */ ] = func() { setA(lsr(cpu.a)); cost(1) }
	op[0x6A /* ROR | 2 */ ] = func() { setA(ror(cpu.a)); cost(1) }
	op[0x8A /* TXA | 2 */ ] = func() { setA(cpu.x); cost(1) }
	op[0xAA /* TAX | 2 */ ] = func() { setX(cpu.a); cost(1) }
	op[0xCA /* DEX | 2 */ ] = func() { setX(cpu.x - 1); cost(1) }
	op[0xEA /* NOP | 2 */ ] = func() { cost(1) }

	op[0x0C /* NOP | 4 */ ] = func() { cost(3) }
	op[0x2C /* BIT | 4 */ ] = func() { bit(read(abs())) }
	op[0x4C /* JMP | 3 */ ] = func() { setPC(abs()) }
	op[0x6C /* JMP | 5 */ ] = func() { l, h := abs(); lo := read(l, h); setPC(lo, read(l+1, h)) }
	op[0x8C /* STY | 4 */ ] = func() { write(fetch(), fetch(), cpu.y) }
	op[0xAC /* LDY | 4 */ ] = func() { setY(read(abs())) }
	op[0xCC /* CPY | 4 */ ] = func() { cmp(read(abs()), cpu.y) }
	op[0xEC /* CPX | 4 */ ] = func() { cmp(read(abs()), cpu.x) }

	op[0x0D /* ORA | 4 */ ] = func() { setA(cpu.a | read(abs())) }
	op[0x2D /* AND | 4 */ ] = func() { setA(cpu.a & read(abs())) }
	op[0x4D /* EOR | 4 */ ] = func() { setA(cpu.a ^ read(abs())) }
	op[0x6D /* ADC | 4 */ ] = func() { setA(adc(read(abs()))) }
	op[0x8D /* STA | 4 */ ] = func() { write(fetch(), fetch(), cpu.a) }
	op[0xAD /* LDA | 4 */ ] = func() { setA(read(abs())) }
	op[0xCD /* CMP | 4 */ ] = func() { cmp(read(abs()), cpu.a) }
	op[0xED /* SBC | 4 */ ] = func() { setA(sbc(read(abs()))) }

	op[0x0E /* ASL | 6 */ ] = func() { l, h := abs(); b := read(l, h); write(l, h, asl(b)); cost(1) }
	op[0x2E /* ROL | 6 */ ] = func() { l, h := abs(); b := read(l, h); write(l, h, rol(b)); cost(1) }
	op[0x4E /* LSR | 6 */ ] = func() { l, h := abs(); b := read(l, h); write(l, h, lsr(b)); cost(1) }
	op[0x6E /* ROR | 6 */ ] = func() { l, h := abs(); b := read(l, h); write(l, h, ror(b)); cost(1) }
	op[0x8E /* STX | 4 */ ] = func() { write(fetch(), fetch(), cpu.x) }
	op[0xAE /* LDX | 4 */ ] = func() { setX(read(abs())) }
	op[0xCE /* DEC | 6 */ ] = func() { l, h := abs(); b := read(l, h); write(l, h, setNZ(b-1)); cost(1) }
	op[0xEE /* INC | 6 */ ] = func() { l, h := abs(); b := read(l, h); write(l, h, setNZ(b+1)); cost(1) }

	op[0x10 /* BPL | 2** */ ] = func() { branch(!hasF(flagN)) }
	op[0x30 /* BMI | 2** */ ] = func() { branch(hasF(flagN)) }
	op[0x50 /* BVC | 2** */ ] = func() { branch(!hasF(flagV)) }
	op[0x70 /* BVS | 2** */ ] = func() { branch(hasF(flagV)) }
	op[0x90 /* BCC | 2** */ ] = func() { branch(!hasF(flagC)) }
	op[0xB0 /* BCS | 2** */ ] = func() { branch(hasF(flagC)) }
	op[0xD0 /* BNE | 2** */ ] = func() { branch(!hasF(flagZ)) }
	op[0xF0 /* BEQ | 2** */ ] = func() { branch(hasF(flagZ)) }

	op[0x11 /* ORA | 5* */ ] = func() { l, h, c := indY(); setA(cpu.a | read(l, h)); cost(c) }
	op[0x31 /* AND | 5* */ ] = func() { l, h, c := indY(); setA(cpu.a & read(l, h)); cost(c) }
	op[0x51 /* EOR | 5* */ ] = func() { l, h, c := indY(); setA(cpu.a ^ read(l, h)); cost(c) }
	op[0x71 /* ADC | 5* */ ] = func() { l, h, c := indY(); setA(adc(read(l, h))); cost(c) }
	op[0x91 /* STA | 6  */ ] = func() { l, h, _ := indY(); write(l, h, cpu.a); cost(1) }
	op[0xB1 /* LDA | 5* */ ] = func() { l, h, c := indY(); setA(read(l, h)); cost(c) }
	op[0xD1 /* CMP | 5* */ ] = func() { l, h, c := indY(); cmp(read(l, h), cpu.a); cost(c) }
	op[0xF1 /* SBC | 5* */ ] = func() { l, h, c := indY(); setA(sbc(read(l, h))); cost(c) }

	op[0x12 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0x32 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0x52 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0x72 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0x92 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0xB2 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0xD2 /* HLT | 1 */ ] = func() { cpu.halted = true }
	op[0xF2 /* HLT | 1 */ ] = func() { cpu.halted = true }

	op[0x14 /* NOP | 4 */ ] = func() { cost(3) }
	op[0x34 /* NOP | 4 */ ] = func() { cost(3) }
	op[0x54 /* NOP | 4 */ ] = func() { cost(3) }
	op[0x74 /* NOP | 4 */ ] = func() { cost(3) }
	op[0x94 /* STY | 4 */ ] = func() { zwrite(fetch()+cpu.x, cpu.y); cost(1) }
	op[0xB4 /* LDY | 4 */ ] = func() { setY(zread(fetch() + cpu.x)); cost(1) }
	op[0xD4 /* NOP | 4 */ ] = func() { cost(3) }
	op[0xF4 /* NOP | 4 */ ] = func() { cost(3) }

	op[0x15 /* ORA | 4 */ ] = func() { setA(cpu.a | zread(fetch()+cpu.x)); cost(1) }
	op[0x35 /* AND | 4 */ ] = func() { setA(cpu.a & zread(fetch()+cpu.x)); cost(1) }
	op[0x55 /* EOR | 4 */ ] = func() { setA(cpu.a ^ zread(fetch()+cpu.x)); cost(1) }
	op[0x75 /* ADC | 4 */ ] = func() { setA(adc(zread(fetch() + cpu.x))); cost(1) }
	op[0x95 /* STA | 4 */ ] = func() { zwrite(fetch()+cpu.x, cpu.a); cost(1) }
	op[0xB5 /* LDA | 4 */ ] = func() { setA(zread(fetch() + cpu.x)); cost(1) }
	op[0xD5 /* CMP | 4 */ ] = func() { cmp(zread(fetch()+cpu.x), cpu.a); cost(1) }
	op[0xF5 /* SBC | 4 */ ] = func() { setA(sbc(zread(fetch() + cpu.x))); cost(1) }

	op[0x16 /* ASL | 6 */ ] = func() { l := fetch() + cpu.x; zwrite(l, asl(zread(l))); cost(2) }
	op[0x36 /* ROL | 6 */ ] = func() { l := fetch() + cpu.x; zwrite(l, rol(zread(l))); cost(2) }
	op[0x56 /* LSR | 6 */ ] = func() { l := fetch() + cpu.x; zwrite(l, lsr(zread(l))); cost(2) }
	op[0x76 /* ROR | 6 */ ] = func() { l := fetch() + cpu.x; zwrite(l, ror(zread(l))); cost(2) }
	op[0x96 /* STX | 4 */ ] = func() { zwrite(fetch()+cpu.y, cpu.x); cost(1) }
	op[0xB6 /* LDX | 4 */ ] = func() { setX(zread(fetch() + cpu.y)); cost(1) }
	op[0xD6 /* DEC | 6 */ ] = func() { l := fetch() + cpu.x; zwrite(l, setNZ(zread(l)-1)); cost(2) }
	op[0xF6 /* INC | 6 */ ] = func() { l := fetch() + cpu.x; zwrite(l, setNZ(zread(l)+1)); cost(2) }

	op[0x18 /* CLC | 2 */ ] = func() { setC(false); cost(1) }
	op[0x38 /* SEC | 2 */ ] = func() { setC(true); cost(1) }
	op[0x58 /* CLI | 2 */ ] = func() { setI(false); cost(1) }
	op[0x78 /* SEI | 2 */ ] = func() { setI(true); cost(1) }
	op[0x98 /* TYA | 2 */ ] = func() { setA(cpu.y); cost(1) }
	op[0xB8 /* CLV | 2 */ ] = func() { setF(false, flagV); cost(1) }
	op[0xD8 /* CLD | 2 */ ] = func() { setF(false, flagD); cost(1) }
	op[0xF8 /* SED | 2 */ ] = func() { setF(true, flagD); cost(1) }

	op[0x19 /* ORA | 4* */ ] = func() { l, h, c := absN(cpu.y); setA(cpu.a | read(l, h)); cost(c) }
	op[0x39 /* AND | 4* */ ] = func() { l, h, c := absN(cpu.y); setA(cpu.a & read(l, h)); cost(c) }
	op[0x59 /* EOR | 4* */ ] = func() { l, h, c := absN(cpu.y); setA(cpu.a ^ read(l, h)); cost(c) }
	op[0x79 /* ADC | 4* */ ] = func() { l, h, c := absN(cpu.y); setA(adc(read(l, h))); cost(c) }
	op[0x99 /* STA | 5  */ ] = func() { l, h, _ := absN(cpu.y); write(l, h, cpu.a); cost(1) }
	op[0xB9 /* LDA | 4* */ ] = func() { l, h, c := absN(cpu.y); setA(read(l, h)); cost(c) }
	op[0xD9 /* CMP | 4* */ ] = func() { l, h, c := absN(cpu.y); cmp(read(l, h), cpu.a); cost(c) }
	op[0xF9 /* SBC | 4* */ ] = func() { l, h, c := absN(cpu.y); setA(sbc(read(l, h))); cost(c) }

	op[0x1A /* NOP | 2 */ ] = func() { cost(1) }
	op[0x3A /* NOP | 2 */ ] = func() { cost(1) }
	op[0x5A /* NOP | 2 */ ] = func() { cost(1) }
	op[0x7A /* NOP | 2 */ ] = func() { cost(1) }
	op[0x9A /* TXS | 2 */ ] = func() { cpu.s = cpu.x; cost(1) }
	op[0xBA /* TSX | 2 */ ] = func() { setX(cpu.s); cost(1) }
	op[0xDA /* NOP | 2 */ ] = func() { cost(1) }
	op[0xFA /* NOP | 2 */ ] = func() { cost(1) }

	op[0x1C /* NOP | 4* */ ] = func() { cost(3) }
	op[0x3C /* NOP | 4* */ ] = func() { cost(3) }
	op[0x5C /* NOP | 4* */ ] = func() { cost(3) }
	op[0x7C /* NOP | 4* */ ] = func() { cost(3) }
	op[0x9C /*     | 1  */ ] = nil
	op[0xBC /* LDY | 4* */ ] = func() { l, h, c := absN(cpu.x); setY(read(l, h)); cost(c) }
	op[0xDC /* NOP | 4* */ ] = func() { cost(3) }
	op[0xFC /* NOP | 4* */ ] = func() { cost(3) }

	op[0x1D /* ORA | 4* */ ] = func() { l, h, c := absN(cpu.x); setA(cpu.a | read(l, h)); cost(c) }
	op[0x3D /* AND | 4* */ ] = func() { l, h, c := absN(cpu.x); setA(cpu.a & read(l, h)); cost(c) }
	op[0x5D /* EOR | 4* */ ] = func() { l, h, c := absN(cpu.x); setA(cpu.a ^ read(l, h)); cost(c) }
	op[0x7D /* ADC | 4* */ ] = func() { l, h, c := absN(cpu.x); setA(adc(read(l, h))); cost(c) }
	op[0x9D /* STA | 5  */ ] = func() { l, h, _ := absN(cpu.x); write(l, h, cpu.a); cost(1) }
	op[0xBD /* LDA | 4* */ ] = func() { l, h, c := absN(cpu.x); setA(read(l, h)); cost(c) }
	op[0xDD /* CMP | 4* */ ] = func() { l, h, c := absN(cpu.x); cmp(read(l, h), cpu.a); cost(c) }
	op[0xFD /* SBC | 4* */ ] = func() { l, h, c := absN(cpu.x); setA(sbc(read(l, h))); cost(c) }

	op[0x1E /* ASL | 7  */ ] = func() { l, h, _ := absN(cpu.x); write(l, h, asl(read(l, h))); cost(2) }
	op[0x3E /* ROL | 7  */ ] = func() { l, h, _ := absN(cpu.x); write(l, h, rol(read(l, h))); cost(2) }
	op[0x5E /* LSR | 7  */ ] = func() { l, h, _ := absN(cpu.x); write(l, h, lsr(read(l, h))); cost(2) }
	op[0x7E /* ROR | 7  */ ] = func() { l, h, _ := absN(cpu.x); write(l, h, ror(read(l, h))); cost(2) }
	op[0x9E /*     | 1  */ ] = nil
	op[0xBE /* LDX | 4* */ ] = func() { l, h, c := absN(cpu.y); setX(read(l, h)); cost(c) }
	op[0xDE /* DEC | 7  */ ] = func() { l, h, _ := absN(cpu.x); write(l, h, setNZ(read(l, h)-1)); cost(2) }
	op[0xFE /* INC | 7  */ ] = func() { l, h, _ := absN(cpu.x); write(l, h, setNZ(read(l, h)+1)); cost(2) }

	// ---

	code := fetch() /* cost 1 */
	if op[code] == nil {
		return fmt.Errorf("invalid op code: %02X%02X: %02X", pch, pcl, code)
	}
	if op[code](); cpu.halted {
		return ErrHalted
	}
	return nil
}

func (f *flag) set(cond bool, bit flag) *flag {
	if cond {
		*f |= bit
	} else {
		*f &= ^bit
	}
	return f
}

func (f *flag) has(bit flag) bool {
	return *f&bit != 0
}

func (f *flag) String() string {
	isset := func(flag flag, char byte) byte {
		if flag != 0 {
			return char
		}
		return '-'
	}
	buf := [6]byte{}
	buf[0] = isset(*f&flagN, 'N')
	buf[1] = isset(*f&flagV, 'V')
	buf[2] = isset(*f&flagD, 'D')
	buf[3] = isset(*f&flagI, 'I')
	buf[4] = isset(*f&flagZ, 'Z')
	buf[5] = isset(*f&flagC, 'C')

	return string(buf[:])
}
