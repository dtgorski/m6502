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
func (cpu *CPU) NMI() {
	cpu.interrupt(
		cpu.bus.Read(0xFA, 0xFF),
		cpu.bus.Read(0xFB, 0xFF),
	)
}

// IRQ processes an interrupt request.
func (cpu *CPU) IRQ() {
	if !cpu.p.has(flagI) {
		cpu.interrupt(
			cpu.bus.Read(0xFE, 0xFF),
			cpu.bus.Read(0xFF, 0xFF),
		)
	}
}

func (cpu *CPU) interrupt(l, h byte) {
	cpu.bus.Write(cpu.s, 0x01, cpu.pch)
	cpu.s--
	cpu.bus.Write(cpu.s, 0x01, cpu.pcl)
	cpu.s--
	cpu.bus.Write(cpu.s, 0x01, byte(*cpu.p|flagU))
	cpu.s--
	cpu.pcl, cpu.pch = l, h
	*cpu.p |= flagI
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
		"m6502: PC=%02X%02X A=%02X X=%02X Y=%02X [%s] S=%02X",
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
	//
	//   Op     | Mnemonic     |  Addressing  |  Processor Flags  | Cycles
	//
	switch fetch() /* cost 1 */ {
	case 0x00: /* BRK          |   implied    | N- Z- C- I+ D- V- | 7 */
		fetch()
		pushPC()
		php()
		setPC(vread(0xFE))
		setI(true)
	case 0x20: /* JSR oper     |   absolute   | N- Z- C- I- D- V- | 6  */
		l := fetch()
		pushPC()
		setPC(l, fetch())
		cost(1)
	case 0x40: /* RTI          |   implied    |    from stack     | 7 */
		plp()
		setPC(popPC())
		cost(3)
	case 0x60: /* RTS          |   implied    | N- Z- C- I- D- V- | 6 */
		setPC(inc(popPC()))
		cost(3)
	case 0x80: /* NOP          |  immediate   | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0xA0: /* LDY #oper    |  immediate   | N+ Z+ C- I- D- V- | 2 */
		setY(fetch())
	case 0xC0: /* CPY #oper    |  immediate   | N+ Z+ C+ I- D- V- | 2 */
		cmp(fetch(), cpu.y)
	case 0xE0: /* CPX #oper    |  immediate   | N+ Z+ C+ I- D- V- | 2 */
		cmp(fetch(), cpu.x)

	case 0x01: /* ORA (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- | 6 */
		setA(cpu.a | read(indX()))
		cost(1)
	case 0x21: /* AND (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- | 6 */
		setA(cpu.a & read(indX()))
		cost(1)
	case 0x41: /* EOR (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- | 6 */
		setA(cpu.a ^ read(indX()))
		cost(1)
	case 0x61: /* ADC (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V+ | 6 */
		setA(adc(read(indX())))
		cost(1)
	case 0x81: /* STA (oper,X) | (indirect,X) | N- Z- C- I- D- V- | 6 */
		l, h := indX()
		write(l, h, cpu.a)
		cost(1)
	case 0xA1: /* LDA (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- | 6 */
		setA(read(indX()))
		cost(1)
	case 0xC1: /* CMP (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V- | 6 */
		cmp(read(indX()), cpu.a)
		cost(1)
	case 0xE1: /* SBC (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V+ | 6 */
		setA(sbc(read(indX())))
		cost(1)

	case 0x02: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0x22: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0x42: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0x62: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0x82: /* NOP          |  immediate   | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0xA2: /* LDX #oper    |  immediate   | N+ Z+ C- I- D- V- | 2 */
		setX(fetch())
	case 0xC2: /* NOP          |  immediate   | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0xE2: /* NOP          |  immediate   | N- Z- C- I- D- V- | 2 */
		cost(1)

	case 0x04: /* NOP          |   zeropage   | N- Z- C- I- D- V- | 3 */
		cost(2)
	case 0x24: /* BIT oper     |   zeropage   | N+ Z+ C- I- D- V+ | 3 */
		bit(zread(fetch()))
	case 0x44: /* NOP          |   zeropage   | N- Z- C- I- D- V- | 3 */
		cost(2)
	case 0x64: /* NOP          |   zeropage   | N- Z- C- I- D- V- | 3 */
		cost(2)
	case 0x84: /* STY oper     |   zeropage   | N- Z- C- I- D- V- | 3 */
		zwrite(fetch(), cpu.y)
	case 0xA4: /* LDY oper     |   zeropage   | N+ Z+ C- I- D- V- | 3 */
		setY(zread(fetch()))
	case 0xC4: /* CPY oper     |   zeropage   | N+ Z+ C+ I- D- V- | 3 */
		cmp(zread(fetch()), cpu.y)
	case 0xE4: /* CPX oper     |   zeropage   | N+ Z+ C+ I- D- V- | 3 */
		cmp(zread(fetch()), cpu.x)

	case 0x05: /* ORA oper     |   zeropage   | N+ Z+ C- I- D- V- | 3 */
		setA(cpu.a | zread(fetch()))
	case 0x25: /* AND oper     |   zeropage   | N+ Z+ C- I- D- V- | 3 */
		setA(cpu.a & zread(fetch()))
	case 0x45: /* EOR oper     |   zeropage   | N+ Z+ C- I- D- V- | 3 */
		setA(cpu.a ^ zread(fetch()))
	case 0x65: /* ADC oper     |   zeropage   | N+ Z+ C+ I- D- V+ | 3 */
		setA(adc(zread(fetch())))
	case 0x85: /* STA oper     |   zeropage   | N- Z- C- I- D- V- | 3 */
		zwrite(fetch(), cpu.a)
	case 0xA5: /* LDA oper     |   zeropage   | N+ Z+ C- I- D- V- | 3 */
		setA(zread(fetch()))
	case 0xC5: /* CMP oper     |   zeropage   | N+ Z+ C+ I- D- V- | 3 */
		cmp(zread(fetch()), cpu.a)
	case 0xE5: /* SBC oper     |   zeropage   | N+ Z+ C+ I- D- V+ | 3 */
		setA(sbc(zread(fetch())))

	case 0x06: /* ASL oper     |   zeropage   | N+ Z+ C+ I- D- V- | 5 */
		b := fetch()
		zwrite(b, asl(zread(b)))
		cost(1)
	case 0x26: /* ROL oper     |   zeropage   | N+ Z+ C+ I- D- V- | 5 */
		b := fetch()
		zwrite(b, rol(zread(b)))
		cost(1)
	case 0x46: /* LSR oper     |   zeropage   | N0 Z+ C+ I- D- V- | 5 */
		b := fetch()
		zwrite(b, lsr(zread(b)))
		cost(1)
	case 0x66: /* ROR oper     |   zeropage   | N+ Z+ C+ I- D- V- | 5 */
		b := fetch()
		zwrite(b, ror(zread(b)))
		cost(1)
	case 0x86: /* STX oper     |   zeropage   | N- Z- C- I- D- V- | 3 */
		zwrite(fetch(), cpu.x)
	case 0xA6: /* LDX oper     |   zeropage   | N+ Z+ C- I- D- V- | 3 */
		setX(zread(fetch()))
	case 0xC6: /* DEC oper     |   zeropage   | N+ Z+ C- I- D- V- | 5 */
		b := fetch()
		zwrite(b, setNZ(zread(b)-1))
		cost(1)
	case 0xE6: /* INC oper     |   zeropage   | N+ Z+ C- I- D- V- | 5 */
		b := fetch()
		zwrite(b, setNZ(zread(b)+1))
		cost(1)

	case 0x08: /* PHP          |   implied    | N- Z- C- I- D- V- | 3 */
		php()
		cost(1)
	case 0x28: /* PLP          |   implied    |    from stack     | 4 */
		plp()
		cost(2)
	case 0x48: /* PHA          |   implied    | N- Z- C- I- D- V- | 3 */
		push(cpu.a)
		cost(1)
	case 0x68: /* PLA          |   implied    | N+ Z+ C- I- D- V- | 4 */
		setA(pop())
		cost(2)
	case 0x88: /* DEY          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setY(cpu.y - 1)
		cost(1)
	case 0xA8: /* TAY          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setY(cpu.a)
		cost(1)
	case 0xC8: /* INY          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setY(cpu.y + 1)
		cost(1)
	case 0xE8: /* INX          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setX(cpu.x + 1)
		cost(1)

	case 0x09: /* ORA #oper    |  immediate   | N+ Z+ C- I- D- V- | 2 */
		setA(cpu.a | fetch())
	case 0x29: /* AND #oper    |  immediate   | N+ Z+ C- I- D- V- | 2 */
		setA(cpu.a & fetch())
	case 0x49: /* EOR #oper    |  immediate   | N+ Z+ C- I- D- V- | 2 */
		setA(cpu.a ^ fetch())
	case 0x69: /* ADC #oper    |  immediate   | N+ Z+ C+ I- D- V+ | 2 */
		setA(adc(fetch()))
	case 0x89: /* NOP          |  immediate   | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0xA9: /* LDA #oper    |  immediate   | N+ Z+ C- I- D- V- | 2 */
		setA(fetch())
	case 0xC9: /* CMP #oper    |  immediate   | N+ Z+ C+ I- D- V- | 2 */
		cmp(fetch(), cpu.a)
	case 0xE9: /* SBC #oper    |  immediate   | N+ Z+ C+ I- D- V+ | 2 */
		setA(sbc(fetch()))

	case 0x0A: /* ASL A        | accumulator  | N+ Z+ C+ I- D- V- | 2 */
		setA(asl(cpu.a))
		cost(1)
	case 0x2A: /* ROL A        | accumulator  | N+ Z+ C+ I- D- V- | 2 */
		setA(rol(cpu.a))
		cost(1)
	case 0x4A: /* LSR A        | accumulator  | N0 Z+ C+ I- D- V- | 2 */
		setA(lsr(cpu.a))
		cost(1)
	case 0x6A: /* ROR A        | accumulator  | N+ Z+ C+ I- D- V- | 2 */
		setA(ror(cpu.a))
		cost(1)
	case 0x8A: /* TXA          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setA(cpu.x)
		cost(1)
	case 0xAA: /* TAX          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setX(cpu.a)
		cost(1)
	case 0xCA: /* DEX          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setX(cpu.x - 1)
		cost(1)
	case 0xEA: /* NOP          |   implied    | N- Z- C- I- D- V- | 2 */
		cost(1)

	case 0x0C: /* NOP          |   absolute   | N- Z- C- I- D- V- | 4 */
		cost(3)
	case 0x2C: /* BIT oper     |   absolute   | N+ Z+ C- I- D- V+ | 4 */
		bit(read(abs()))
	case 0x4C: /* JMP oper     |   absolute   | N- Z- C- I- D- V- | 3 */
		setPC(abs())
	case 0x6C: /* JMP (oper)   |   indirect   | N- Z- C- I- D- V- | 5 */
		l, h := abs()
		lo := read(l, h)
		setPC(lo, read(l+1, h))
	case 0x8C: /* STY oper     |   absolute   | N- Z- C- I- D- V- | 4 */
		write(fetch(), fetch(), cpu.y)
	case 0xAC: /* LDY oper     |   absolute   | N+ Z+ C- I- D- V- | 4 */
		setY(read(abs()))
	case 0xCC: /* CPY oper     |   absolute   | N+ Z+ C+ I- D- V- | 4 */
		cmp(read(abs()), cpu.y)
	case 0xEC: /* CPX oper     |   absolute   | N+ Z+ C+ I- D- V- | 4 */
		cmp(read(abs()), cpu.x)

	case 0x0D: /* ORA oper     |   absolute   | N+ Z+ C- I- D- V- | 4 */
		setA(cpu.a | read(abs()))
	case 0x2D: /* AND oper     |   absolute   | N+ Z+ C- I- D- V- | 4 */
		setA(cpu.a & read(abs()))
	case 0x4D: /* EOR oper     |   absolute   | N+ Z+ C- I- D- V- | 4 */
		setA(cpu.a ^ read(abs()))
	case 0x6D: /* ADC oper     |   absolute   | N+ Z+ C+ I- D- V+ | 4 */
		setA(adc(read(abs())))
	case 0x8D: /* STA oper     |   absolute   | N- Z- C- I- D- V- | 4 */
		write(fetch(), fetch(), cpu.a)
	case 0xAD: /* LDA oper     |   absolute   | N+ Z+ C- I- D- V- | 4 */
		setA(read(abs()))
	case 0xCD: /* CMP oper     |   absolute   | N+ Z+ C+ I- D- V- | 4 */
		cmp(read(abs()), cpu.a)
	case 0xED: /* SBC oper     |   absolute   | N+ Z+ C+ I- D- V+ | 4 */
		setA(sbc(read(abs())))

	case 0x0E: /* ASL oper     |   absolute   | N+ Z+ C+ I- D- V- | 6 */
		l, h := abs()
		b := read(l, h)
		write(l, h, asl(b))
		cost(1)
	case 0x2E: /* ROL oper     |   absolute   | N+ Z+ C+ I- D- V- | 6 */
		l, h := abs()
		b := read(l, h)
		write(l, h, rol(b))
		cost(1)
	case 0x4E: /* LSR oper     |   absolute   | N0 Z+ C+ I- D- V- | 6 */
		l, h := abs()
		b := read(l, h)
		write(l, h, lsr(b))
		cost(1)
	case 0x6E: /* ROR oper     |   absolute   | N+ Z+ C+ I- D- V- | 6 */
		l, h := abs()
		b := read(l, h)
		write(l, h, ror(b))
		cost(1)
	case 0x8E: /* STX oper     |   absolute   | N- Z- C- I- D- V- | 4 */
		write(fetch(), fetch(), cpu.x)
	case 0xAE: /* LDX oper     |   absolute   | N+ Z+ C- I- D- V- | 4 */
		setX(read(abs()))
	case 0xCE: /* DEC oper     |   absolute   | N+ Z+ C- I- D- V- | 6 */
		l, h := abs()
		b := read(l, h)
		write(l, h, setNZ(b-1))
		cost(1)
	case 0xEE: /* INC oper     |   absolute   | N+ Z+ C- I- D- V- | 6 */
		l, h := abs()
		b := read(l, h)
		write(l, h, setNZ(b+1))
		cost(1)

	case 0x10: /* BPL oper     |   relative   | N- Z- C- I- D- V- | 2** */
		branch(!hasF(flagN))
	case 0x30: /* BMI oper     |   relative   | N- Z- C- I- D- V- | 2** */
		branch(hasF(flagN))
	case 0x50: /* BVC oper     |   relative   | N- Z- C- I- D- V- | 2** */
		branch(!hasF(flagV))
	case 0x70: /* BVS oper     |   relative   | N- Z- C- I- D- V- | 2** */
		branch(hasF(flagV))
	case 0x90: /* BCC oper     |   relative   | N- Z- C- I- D- V- | 2** */
		branch(!hasF(flagC))
	case 0xB0: /* BCS oper     |   relative   | N- Z- C- I- D- V- | 2** */
		branch(hasF(flagC))
	case 0xD0: /* BNE oper     |   relative   | N- Z- C- I- D- V- | 2** */
		branch(!hasF(flagZ))
	case 0xF0: /* BEQ oper     |   relative   | N- Z- C- I- D- V- | 2** */
		branch(hasF(flagZ))

	case 0x11: /* ORA (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- | 5* */
		l, h, c := indY()
		setA(cpu.a | read(l, h))
		cost(c)
	case 0x31: /* AND (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- | 5* */
		l, h, c := indY()
		setA(cpu.a & read(l, h))
		cost(c)
	case 0x51: /* EOR (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- | 5* */
		l, h, c := indY()
		setA(cpu.a ^ read(l, h))
		cost(c)
	case 0x71: /* ADC (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V+ | 5* */
		l, h, c := indY()
		setA(adc(read(l, h)))
		cost(c)
	case 0x91: /* STA (oper),Y | (indirect),Y | N- Z- C- I- D- V- | 6 */
		l, h, _ := indY()
		write(l, h, cpu.a)
		cost(1)
	case 0xB1: /* LDA (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- | 5* */
		l, h, c := indY()
		setA(read(l, h))
		cost(c)
	case 0xD1: /* CMP (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V- | 5* */
		l, h, c := indY()
		cmp(read(l, h), cpu.a)
		cost(c)
	case 0xF1: /* SBC (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V+ | 5* */
		l, h, c := indY()
		setA(sbc(read(l, h)))
		cost(c)

	case 0x12: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0x32: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0x52: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0x72: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0x92: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0xB2: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0xD2: /* HLT          |              |                   | 1 */
		cpu.halted = true
	case 0xF2: /* HLT          |              |                   | 1 */
		cpu.halted = true

	case 0x14: /* NOP          |  zeropage,X  | N- Z- C- I- D- V- | 4 */
		cost(3)
	case 0x34: /* NOP          |  zeropage,X  | N- Z- C- I- D- V- | 4 */
		cost(3)
	case 0x54: /* NOP          |  zeropage,X  | N- Z- C- I- D- V- | 4 */
		cost(3)
	case 0x74: /* NOP          |  zeropage,X  | N- Z- C- I- D- V- | 4 */
		cost(3)
	case 0x94: /* STY oper,X   |  zeropage,X  | N- Z- C- I- D- V- | 4 */
		zwrite(fetch()+cpu.x, cpu.y)
		cost(1)
	case 0xB4: /* LDY oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- | 4 */
		setY(zread(fetch() + cpu.x))
		cost(1)
	case 0xD4: /* NOP          |  zeropage,X  | N- Z- C- I- D- V- | 4 */
		cost(3)
	case 0xF4: /* NOP          |  zeropage,X  | N- Z- C- I- D- V- | 4 */
		cost(3)

	case 0x15: /* ORA oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- | 4 */
		setA(cpu.a | zread(fetch()+cpu.x))
		cost(1)
	case 0x35: /* AND oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- | 4 */
		setA(cpu.a & zread(fetch()+cpu.x))
		cost(1)
	case 0x55: /* EOR oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- | 4 */
		setA(cpu.a ^ zread(fetch()+cpu.x))
		cost(1)
	case 0x75: /* ADC oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V+ | 4 */
		setA(adc(zread(fetch() + cpu.x)))
		cost(1)
	case 0x95: /* STA oper,X   |  zeropage,X  | N- Z- C- I- D- V- | 4 */
		zwrite(fetch()+cpu.x, cpu.a)
		cost(1)
	case 0xB5: /* LDA oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- | 4 */
		setA(zread(fetch() + cpu.x))
		cost(1)
	case 0xD5: /* CMP oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V- | 4 */
		cmp(zread(fetch()+cpu.x), cpu.a)
		cost(1)
	case 0xF5: /* SBC oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V+ | 4 */
		setA(sbc(zread(fetch() + cpu.x)))
		cost(1)

	case 0x16: /* ASL oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V- | 6 */
		l := fetch() + cpu.x
		zwrite(l, asl(zread(l)))
		cost(2)
	case 0x36: /* ROL oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V- | 6 */
		l := fetch() + cpu.x
		zwrite(l, rol(zread(l)))
		cost(2)
	case 0x56: /* LSR oper,X   |  zeropage,X  | N0 Z+ C+ I- D- V- | 6 */
		l := fetch() + cpu.x
		zwrite(l, lsr(zread(l)))
		cost(2)
	case 0x76: /* ROR oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V- | 6 */
		l := fetch() + cpu.x
		zwrite(l, ror(zread(l)))
		cost(2)
	case 0x96: /* STX oper,Y   |  zeropage,Y  | N- Z- C- I- D- V- | 4 */
		zwrite(fetch()+cpu.y, cpu.x)
		cost(1)
	case 0xB6: /* LDX oper,Y   |  zeropage,Y  | N+ Z+ C- I- D- V- | 4 */
		setX(zread(fetch() + cpu.y))
		cost(1)
	case 0xD6: /* DEC oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- | 6 */
		l := fetch() + cpu.x
		zwrite(l, setNZ(zread(l)-1))
		cost(2)
	case 0xF6: /* INC oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- | 6 */
		l := fetch() + cpu.x
		zwrite(l, setNZ(zread(l)+1))
		cost(2)

	case 0x18: /* CLC          |   implied    | N- Z- C0 I- D- V- | 2 */
		setC(false)
		cost(1)
	case 0x38: /* SEC          |   implied    | N- Z- C1 I- D- V- | 2 */
		setC(true)
		cost(1)
	case 0x58: /* CLI          |   implied    | N- Z- C- I0 D- V- | 2 */
		setI(false)
		cost(1)
	case 0x78: /* SEI          |   implied    | N- Z- C- I1 D- V- | 2 */
		setI(true)
		cost(1)
	case 0x98: /* TYA          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setA(cpu.y)
		cost(1)
	case 0xB8: /* CLV          |   implied    | N- Z- C- I- D- V0 | 2 */
		setF(false, flagV)
		cost(1)
	case 0xD8: /* CLD          |   implied    | N- Z- C- I- D0 V- | 2 */
		setF(false, flagD)
		cost(1)
	case 0xF8: /* SED          |   implied    | N- Z- C- I- D1 V- | 2 */
		setF(true, flagD)
		cost(1)

	case 0x19: /* ORA oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.y)
		setA(cpu.a | read(l, h))
		cost(c)
	case 0x39: /* AND oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.y)
		setA(cpu.a & read(l, h))
		cost(c)
	case 0x59: /* EOR oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.y)
		setA(cpu.a ^ read(l, h))
		cost(c)
	case 0x79: /* ADC oper,Y   |  absolute,Y  | N+ Z+ C+ I- D- V+ | 4* */
		l, h, c := absN(cpu.y)
		setA(adc(read(l, h)))
		cost(c)
	case 0x99: /* STA oper,Y   |  absolute,Y  | N- Z- C- I- D- V- | 5 */
		l, h, _ := absN(cpu.y)
		write(l, h, cpu.a)
		cost(1)
	case 0xB9: /* LDA oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.y)
		setA(read(l, h))
		cost(c)
	case 0xD9: /* CMP oper,Y   |  absolute,Y  | N+ Z+ C+ I- D- V- | 4* */
		l, h, c := absN(cpu.y)
		cmp(read(l, h), cpu.a)
		cost(c)
	case 0xF9: /* SBC oper,Y   |  absolute,Y  | N+ Z+ C+ I- D- V+ | 4* */
		l, h, c := absN(cpu.y)
		setA(sbc(read(l, h)))
		cost(c)

	case 0x1A: /* NOP          |   implied    | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0x3A: /* NOP          |   implied    | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0x5A: /* NOP          |   implied    | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0x7A: /* NOP          |   implied    | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0x9A: /* TXS          |   implied    | N- Z- C- I- D- V- | 2 */
		cpu.s = cpu.x
		cost(1)
	case 0xBA: /* TSX          |   implied    | N+ Z+ C- I- D- V- | 2 */
		setX(cpu.s)
		cost(1)
	case 0xDA: /* NOP          |   implied    | N- Z- C- I- D- V- | 2 */
		cost(1)
	case 0xFA: /* NOP          |   implied    | N- Z- C- I- D- V- | 2 */
		cost(1)

	case 0x1C: /* NOP          |  absolute,X  | N- Z- C- I- D- V- | 4* */
		cost(3)
	case 0x3C: /* NOP          |  absolute,X  | N- Z- C- I- D- V- | 4* */
		cost(3)
	case 0x5C: /* NOP          |  absolute,X  | N- Z- C- I- D- V- | 4* */
		cost(3)
	case 0x7C: /* NOP          |  absolute,X  | N- Z- C- I- D- V- | 4* */
		cost(3)
	case 0xBC: /* LDY oper,X   |  absolute,X  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.x)
		setY(read(l, h))
		cost(c)
	case 0xDC: /* NOP          |  absolute,X  | N- Z- C- I- D- V- | 4* */
		cost(3)
	case 0xFC: /* NOP          |  absolute,X  | N- Z- C- I- D- V- | 4* */
		cost(3)

	case 0x1D: /* ORA oper,X   |  absolute,X  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.x)
		setA(cpu.a | read(l, h))
		cost(c)
	case 0x3D: /* AND oper,X   |  absolute,X  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.x)
		setA(cpu.a & read(l, h))
		cost(c)
	case 0x5D: /* EOR oper,X   |  absolute,X  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.x)
		setA(cpu.a ^ read(l, h))
		cost(c)
	case 0x7D: /* ADC oper,X   |  absolute,X  | N+ Z+ C+ I- D- V+ | 4* */
		l, h, c := absN(cpu.x)
		setA(adc(read(l, h)))
		cost(c)
	case 0x9D: /* STA oper,X   |  absolute,X  | N- Z- C- I- D- V- | 5 */
		l, h, _ := absN(cpu.x)
		write(l, h, cpu.a)
		cost(1)
	case 0xBD: /* LDA oper,X   |  absolute,X  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.x)
		setA(read(l, h))
		cost(c)
	case 0xDD: /* CMP oper,X   |  absolute,X  | N+ Z+ C+ I- D- V- | 4* */
		l, h, c := absN(cpu.x)
		cmp(read(l, h), cpu.a)
		cost(c)
	case 0xFD: /* SBC oper,X   |  absolute,X  | N+ Z+ C+ I- D- V+ | 4* */
		l, h, c := absN(cpu.x)
		setA(sbc(read(l, h)))
		cost(c)

	case 0x1E: /* ASL oper,X   |  absolute,X  | N+ Z+ C+ I- D- V- | 7 */
		l, h, _ := absN(cpu.x)
		write(l, h, asl(read(l, h)))
		cost(2)
	case 0x3E: /* ROL oper,X   |  absolute,X  | N+ Z+ C+ I- D- V- | 7 */
		l, h, _ := absN(cpu.x)
		write(l, h, rol(read(l, h)))
		cost(2)
	case 0x5E: /* LSR oper,X   |  absolute,X  | N0 Z+ C+ I- D- V- | 7 */
		l, h, _ := absN(cpu.x)
		write(l, h, lsr(read(l, h)))
		cost(2)
	case 0x7E: /* ROR oper,X   |  absolute,X  | N+ Z+ C+ I- D- V- | 7 */
		l, h, _ := absN(cpu.x)
		write(l, h, ror(read(l, h)))
		cost(2)
	case 0xBE: /* LDX oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- | 4* */
		l, h, c := absN(cpu.y)
		setX(read(l, h))
		cost(c)
	case 0xDE: /* DEC oper,X   |  absolute,X  | N+ Z+ C- I- D- V- | 7 */
		l, h, _ := absN(cpu.x)
		write(l, h, setNZ(read(l, h)-1))
		cost(2)
	case 0xFE: /* INC oper,X   |  absolute,X  | N+ Z+ C- I- D- V- | 7 */
		l, h, _ := absN(cpu.x)
		write(l, h, setNZ(read(l, h)+1))
		cost(2)
	default:
		return fmt.Errorf("m6502: invalid op code: %02X%02X: %02X", pch, pcl, read(pcl, pch))
	}

	if cpu.halted {
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
