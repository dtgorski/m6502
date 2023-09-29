# m6502 implemented instructions

```
Cycles penalty (column 5):
   * add 1 to cycles if page boundary is crossed
  ** add 1 to cycles if branch occurs on same page
  ** add 2 to cycles if branch occurs to different page
```
---
```
 Op  | Mnemonic     |  Addressing  |  Processor Flags  | Cycles | Description
     |              |              |                   |        |
0x00 | BRK          |   implied    | N- Z- C- I+ D- V- |   7    | Force Break
0x20 | JSR oper     |   absolute   | N- Z- C- I- D- V- |   6    | Jump to New Location Saving Return Address
0x40 | RTI          |   implied    |    from stack     |   7    | Return from Interrupt
0x60 | RTS          |   implied    | N- Z- C- I- D- V- |   6    | Return from Subroutine
0x80 | NOP          |  immediate   | N- Z- C- I- D- V- |   2    | No Operation
0xA0 | LDY #oper    |  immediate   | N+ Z+ C- I- D- V- |   2    | Load Index Y with Memory
0xC0 | CPY #oper    |  immediate   | N+ Z+ C+ I- D- V- |   2    | Compare Memory and Index Y
0xE0 | CPX #oper    |  immediate   | N+ Z+ C+ I- D- V- |   2    | Compare Memory and Index X
     |              |              |                   |        |
0x01 | ORA (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- |   6    | OR Memory with Accumulator
0x21 | AND (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- |   6    | AND Memory with Accumulator
0x41 | EOR (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- |   6    | Exclusive-OR Memory with Accumulator
0x61 | ADC (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V+ |   6    | Add Memory to Accumulator with Carry
0x81 | STA (oper,X) | (indirect,X) | N- Z- C- I- D- V- |   6    | Store Accumulator in Memory
0xA1 | LDA (oper,X) | (indirect,X) | N+ Z+ C- I- D- V- |   6    | Load Accumulator with Memory
0xC1 | CMP (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V- |   6    | Compare Memory with Accumulator
0xE1 | SBC (oper,X) | (indirect,X) | N+ Z+ C+ I- D- V+ |   6    | Subtract Memory from Accumulator with Borrow
     |              |              |                   |        |
0x02 | HLT          |              |                   |   1    | Booboo, Halt
0x22 | HLT          |              |                   |   1    | Booboo, Halt
0x42 | HLT          |              |                   |   1    | Booboo, Halt
0x62 | HLT          |              |                   |   1    | Booboo, Halt
0x82 | NOP          |  immediate   | N- Z- C- I- D- V- |   2    | No Operation
0xA2 | LDX #oper    |  immediate   | N+ Z+ C- I- D- V- |   2    | Load Index X with Memory
0xC2 | NOP          |  immediate   | N- Z- C- I- D- V- |   2    | No Operation
0xE2 | NOP          |  immediate   | N- Z- C- I- D- V- |   2    | No Operation
     |              |              |                   |        |
0x04 | NOP          |   zeropage   | N- Z- C- I- D- V- |   3    | No Operation
0x24 | BIT oper     |   zeropage   | N+ Z+ C- I- D- V+ |   3    | No Operation
0x44 | NOP          |   zeropage   | N- Z- C- I- D- V- |   3    | No Operation
0x64 | NOP          |   zeropage   | N- Z- C- I- D- V- |   3    | No Operation
0x84 | STY oper     |   zeropage   | N- Z- C- I- D- V- |   3    | Store Index Y in Memory
0xA4 | LDY oper     |   zeropage   | N+ Z+ C- I- D- V- |   3    | Load Index Y with Memory
0xC4 | CPY oper     |   zeropage   | N+ Z+ C+ I- D- V- |   3    | Compare Memory and Index Y
0xE4 | CPX oper     |   zeropage   | N+ Z+ C+ I- D- V- |   3    | Compare Memory and Index X
     |              |              |                   |        |
0x05 | ORA oper     |   zeropage   | N+ Z+ C- I- D- V- |   3    | OR Memory with Accumulator
0x25 | AND oper     |   zeropage   | N+ Z+ C- I- D- V- |   3    | AND Memory with Accumulator
0x45 | EOR oper     |   zeropage   | N+ Z+ C- I- D- V- |   3    | Exclusive-OR Memory with Accumulator
0x65 | ADC oper     |   zeropage   | N+ Z+ C+ I- D- V+ |   3    | Add Memory to Accumulator with Carry
0x85 | STA oper     |   zeropage   | N- Z- C- I- D- V- |   3    | Store Accumulator in Memory
0xA5 | LDA oper     |   zeropage   | N+ Z+ C- I- D- V- |   3    | Load Accumulator with Memory
0xC5 | CMP oper     |   zeropage   | N+ Z+ C+ I- D- V- |   3    | Compare Memory with Accumulator
0xE5 | SBC oper     |   zeropage   | N+ Z+ C+ I- D- V+ |   3    | Subtract Memory from Accumulator with Borrow
     |              |              |                   |        |
0x06 | ASL oper     |   zeropage   | N+ Z+ C+ I- D- V- |   5    | Shift Left One Bit (Memory)
0x26 | ROL oper     |   zeropage   | N+ Z+ C+ I- D- V- |   5    | Rotate One Bit Left (Memory)
0x46 | LSR oper     |   zeropage   | N0 Z+ C+ I- D- V- |   5    | Shift One Bit Right (Memory)
0x66 | ROR oper     |   zeropage   | N+ Z+ C+ I- D- V- |   5    | Rotate One Bit Right (Memory)
0x86 | STX oper     |   zeropage   | N- Z- C- I- D- V- |   3    | Store Index X in Memory
0xA6 | LDX oper     |   zeropage   | N+ Z+ C- I- D- V- |   3    | Load Index X with Memory
0xC6 | DEC oper     |   zeropage   | N+ Z+ C- I- D- V- |   5    | Decrement Memory by One
0xE6 | INC oper     |   zeropage   | N+ Z+ C- I- D- V- |   5    | Increment Memory by One
     |              |              |                   |        |
0x08 | PHP          |   implied    | N- Z- C- I- D- V- |   3    | Push Processor Status on Stack
0x28 | PLP          |   implied    |    from stack     |   4    | Pull Processor Status from Stack
0x48 | PHA          |   implied    | N- Z- C- I- D- V- |   3    | Push Accumulator on Stack
0x68 | PLA          |   implied    | N+ Z+ C- I- D- V- |   4    | Pull Accumulator from Stack
0x88 | DEY          |   implied    | N+ Z+ C- I- D- V- |   2    | Decrement Index Y by One
0xA8 | TAY          |   implied    | N+ Z+ C- I- D- V- |   2    | Transfer Accumulator to Index Y
0xC8 | INY          |   implied    | N+ Z+ C- I- D- V- |   2    | Increment Index Y by One
0xE8 | INX          |   implied    | N+ Z+ C- I- D- V- |   2    | Increment Index X by One
     |              |              |                   |        |
0x09 | ORA #oper    |  immediate   | N+ Z+ C- I- D- V- |   2    | OR Memory with Accumulator
0x29 | AND #oper    |  immediate   | N+ Z+ C- I- D- V- |   2    | AND Memory with Accumulator
0x49 | EOR #oper    |  immediate   | N+ Z+ C- I- D- V- |   2    | Exclusive-OR Memory with Accumulator
0x69 | ADC #oper    |  immediate   | N+ Z+ C+ I- D- V+ |   2    | Add Memory to Accumulator with Carry
0x89 | NOP          |  immediate   | N- Z- C- I- D- V- |   2    | No Operation
0xA9 | LDA #oper    |  immediate   | N+ Z+ C- I- D- V- |   2    | Load Accumulator with Memory
0xC9 | CMP #oper    |  immediate   | N+ Z+ C+ I- D- V- |   2    | Compare Memory with Accumulator
0xE9 | SBC #oper    |  immediate   | N+ Z+ C+ I- D- V+ |   2    | Subtract Memory from Accumulator with Borrow
     |              |              |                   |        |
0x0A | ASL A        | accumulator  | N+ Z+ C+ I- D- V- |   2    | Shift Left One Bit (Accumulator)
0x2A | ROL A        | accumulator  | N+ Z+ C+ I- D- V- |   2    | Rotate One Bit Left (Accumulator)
0x4A | LSR A        | accumulator  | N0 Z+ C+ I- D- V- |   2    | Shift One Bit Right (Accumulator)
0x6A | ROR A        | accumulator  | N+ Z+ C+ I- D- V- |   2    | Rotate One Bit Right (Accumulator)
0x8A | TXA          |   implied    | N+ Z+ C- I- D- V- |   2    | Transfer Index X to Accumulator
0xAA | TAX          |   implied    | N+ Z+ C- I- D- V- |   2    | Transfer Accumulator to Index X
0xCA | DEX          |   implied    | N+ Z+ C- I- D- V- |   2    | Decrement Index X by One
0xEA | NOP          |   implied    | N- Z- C- I- D- V- |   2    | No Operation
     |              |              |                   |        |
0x0C | NOP          |   absolute   | N- Z- C- I- D- V- |   4    | No Operation
0x2C | BIT oper     |   absolute   | N+ Z+ C- I- D- V+ |   4    | Test Bits in Memory with Accumulator
0x4C | JMP oper     |   absolute   | N- Z- C- I- D- V- |   3    | Jump to New Location
0x6C | JMP (oper)   |   indirect   | N- Z- C- I- D- V- |   5    | Jump to New Location
0x8C | STY oper     |   absolute   | N- Z- C- I- D- V- |   4    | Store Index Y in Memory
0xAC | LDY oper     |   absolute   | N+ Z+ C- I- D- V- |   4    | Load Index Y with Memory
0xCC | CPY oper     |   absolute   | N+ Z+ C+ I- D- V- |   4    | Compare Memory and Index Y
0xEC | CPX oper     |   absolute   | N+ Z+ C+ I- D- V- |   4    | Compare Memory and Index X
     |              |              |                   |        |
0x0D | ORA oper     |   absolute   | N+ Z+ C- I- D- V- |   4    | OR Memory with Accumulator
0x2D | AND oper     |   absolute   | N+ Z+ C- I- D- V- |   4    | AND Memory with Accumulator
0x4D | EOR oper     |   absolute   | N+ Z+ C- I- D- V- |   4    | Exclusive-OR Memory with Accumulator
0x6D | ADC oper     |   absolute   | N+ Z+ C+ I- D- V+ |   4    | Add Memory to Accumulator with Carry
0x8D | STA oper     |   absolute   | N- Z- C- I- D- V- |   4    | Store Accumulator in Memory
0xAD | LDA oper     |   absolute   | N+ Z+ C- I- D- V- |   4    | Load Accumulator with Memory
0xCD | CMP oper     |   absolute   | N+ Z+ C+ I- D- V- |   4    | Compare Memory with Accumulator
0xED | SBC oper     |   absolute   | N+ Z+ C+ I- D- V+ |   4    | Subtract Memory from Accumulator with Borrow
     |              |              |                   |        |
0x0E | ASL oper     |   absolute   | N+ Z+ C+ I- D- V- |   6    | Shift Left One Bit (Memory)
0x2E | ROL oper     |   absolute   | N+ Z+ C+ I- D- V- |   6    | Rotate One Bit Left (Memory)
0x4E | LSR oper     |   absolute   | N0 Z+ C+ I- D- V- |   6    | Shift One Bit Right (Memory)
0x6E | ROR oper     |   absolute   | N+ Z+ C+ I- D- V- |   6    | Rotate One Bit Right (Memory)
0x8E | STX oper     |   absolute   | N- Z- C- I- D- V- |   4    | Store Index X in Memory
0xAE | LDX oper     |   absolute   | N+ Z+ C- I- D- V- |   4    | Load Index X with Memory
0xCE | DEC oper     |   absolute   | N+ Z+ C- I- D- V- |   6    | Decrement Memory by One
0xEE | INC oper     |   absolute   | N+ Z+ C- I- D- V- |   6    | Increment Memory by One
     |              |              |                   |        |
0x10 | BPL oper     |   relative   | N- Z- C- I- D- V- |  2**   | Branch on Result Plus
0x30 | BMI oper     |   relative   | N- Z- C- I- D- V- |  2**   | Branch on Result Minus
0x50 | BVC oper     |   relative   | N- Z- C- I- D- V- |  2**   | Branch on Overflow Clear
0x70 | BVS oper     |   relative   | N- Z- C- I- D- V- |  2**   | Branch on Overflow Set
0x90 | BCC oper     |   relative   | N- Z- C- I- D- V- |  2**   | Branch on Carry Clear
0xB0 | BCS oper     |   relative   | N- Z- C- I- D- V- |  2**   | Branch on Carry Set
0xD0 | BNE oper     |   relative   | N- Z- C- I- D- V- |  2**   | Branch on Result not Zero
0xF0 | BEQ oper     |   relative   | N- Z- C- I- D- V- |  2**   | Branch on Result Zero
     |              |              |                   |        |
0x11 | ORA (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- |   5*   | OR Memory with Accumulator
0x31 | AND (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- |   5*   | AND Memory with Accumulator
0x51 | EOR (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- |   5*   | Exclusive-OR Memory with Accumulator
0x71 | ADC (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V+ |   5*   | Add Memory to Accumulator with Carry
0x91 | STA (oper),Y | (indirect),Y | N- Z- C- I- D- V- |   6    | Store Accumulator in Memory
0xB1 | LDA (oper),Y | (indirect),Y | N+ Z+ C- I- D- V- |   5*   | Load Accumulator with Memory
0xD1 | CMP (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V- |   5*   | Compare Memory with Accumulator
0xF1 | SBC (oper),Y | (indirect),Y | N+ Z+ C+ I- D- V+ |   5*   | Subtract Memory from Accumulator with Borrow
     |              |              |                   |        |
0x12 | HLT          |              |                   |   1    | Halt
0x32 | HLT          |              |                   |   1    | Halt
0x52 | HLT          |              |                   |   1    | Halt
0x72 | HLT          |              |                   |   1    | Halt
0x92 | HLT          |              |                   |   1    | Halt
0xB2 | HLT          |              |                   |   1    | Halt
0xD2 | HLT          |              |                   |   1    | Halt
0xF2 | HLT          |              |                   |   1    | Halt
     |              |              |                   |        |
0x14 | NOP          |  zeropage,X  | N- Z- C- I- D- V- |   4    | No Operation
0x34 | NOP          |  zeropage,X  | N- Z- C- I- D- V- |   4    | No Operation
0x54 | NOP          |  zeropage,X  | N- Z- C- I- D- V- |   4    | No Operation
0x74 | NOP          |  zeropage,X  | N- Z- C- I- D- V- |   4    | No Operation
0x94 | STY oper,X   |  zeropage,X  | N- Z- C- I- D- V- |   4    | Store Index Y in Memory
0xB4 | LDY oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- |   4    | Load Index Y with Memory
0xD4 | NOP          |  zeropage,X  | N- Z- C- I- D- V- |   4    | No Operation
0xF4 | NOP          |  zeropage,X  | N- Z- C- I- D- V- |   4    | No Operation
     |              |              |                   |        |
0x15 | ORA oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- |   4    | OR Memory with Accumulator
0x35 | AND oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- |   4    | AND Memory with Accumulator
0x55 | EOR oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- |   4    | Exclusive-OR Memory with Accumulator
0x75 | ADC oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V+ |   4    | Add Memory to Accumulator with Carry
0x95 | STA oper,X   |  zeropage,X  | N- Z- C- I- D- V- |   4    | Store Accumulator in Memory
0xB5 | LDA oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- |   4    | Load Accumulator with Memory
0xD5 | CMP oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V- |   4    | Compare Memory with Accumulator
0xF5 | SBC oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V+ |   4    | Subtract Memory from Accumulator with Borrow
     |              |              |                   |        |
0x16 | ASL oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V- |   6    | Shift Left One Bit (Memory)
0x36 | ROL oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V- |   6    | Rotate One Bit Left (Memory)
0x56 | LSR oper,X   |  zeropage,X  | N0 Z+ C+ I- D- V- |   6    | Shift One Bit Right (Memory)
0x76 | ROR oper,X   |  zeropage,X  | N+ Z+ C+ I- D- V- |   6    | Rotate One Bit Right (Memory)
0x96 | STX oper,Y   |  zeropage,Y  | N- Z- C- I- D- V- |   4    | Store Index X in Memory
0xB6 | LDX oper,Y   |  zeropage,Y  | N+ Z+ C- I- D- V- |   4    | Load Index X with Memory
0xD6 | DEC oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- |   6    | Decrement Memory by One
0xF6 | INC oper,X   |  zeropage,X  | N+ Z+ C- I- D- V- |   6    | Increment Memory by One
     |              |              |                   |        |
0x18 | CLC          |   implied    | N- Z- C0 I- D- V- |   2    | Clear Carry Flag
0x38 | SEC          |   implied    | N- Z- C1 I- D- V- |   2    | Set Carry Flag
0x58 | CLI          |   implied    | N- Z- C- I0 D- V- |   2    | Clear Interrupt Disable Bit
0x78 | SEI          |   implied    | N- Z- C- I1 D- V- |   2    | Set Interrupt Disable Status
0x98 | TYA          |   implied    | N+ Z+ C- I- D- V- |   2    | Transfer Index Y to Accumulator
0xB8 | CLV          |   implied    | N- Z- C- I- D- V0 |   2    | Clear Overflow Flag
0xD8 | CLD          |   implied    | N- Z- C- I- D0 V- |   2    | Clear Decimal Mode
0xF8 | SED          |   implied    | N- Z- C- I- D1 V- |   2    | Set Decimal Flag
     |              |              |                   |        |
0x19 | ORA oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- |   4*   | OR Memory with Accumulator
0x39 | AND oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- |   4*   | AND Memory with Accumulator
0x59 | EOR oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- |   4*   | Exclusive-OR Memory with Accumulator
0x79 | ADC oper,Y   |  absolute,Y  | N+ Z+ C+ I- D- V+ |   4*   | Add Memory to Accumulator with Carry
0x99 | STA oper,Y   |  absolute,Y  | N- Z- C- I- D- V- |   5    | Store Accumulator in Memory
0xB9 | LDA oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- |   4*   | Load Accumulator with Memory
0xD9 | CMP oper,Y   |  absolute,Y  | N+ Z+ C+ I- D- V- |   4*   | Compare Memory with Accumulator
0xF9 | SBC oper,Y   |  absolute,Y  | N+ Z+ C+ I- D- V+ |   4*   | Subtract Memory from Accumulator with Borrow
     |              |              |                   |        |
0x1A | NOP          |   implied    | N- Z- C- I- D- V- |   2    | No Operation
0x3A | NOP          |   implied    | N- Z- C- I- D- V- |   2    | No Operation
0x5A | NOP          |   implied    | N- Z- C- I- D- V- |   2    | No Operation
0x7A | NOP          |   implied    | N- Z- C- I- D- V- |   2    | No Operation
0x9A | TXS          |   implied    | N- Z- C- I- D- V- |   2    | Transfer Index X to Stack Register
0xBA | TSX          |   implied    | N+ Z+ C- I- D- V- |   2    | Transfer Stack Pointer to Index X
0xDA | NOP          |   implied    | N- Z- C- I- D- V- |   2    | No Operation
0xFA | NOP          |   implied    | N- Z- C- I- D- V- |   2    | No Operation
     |              |              |                   |        |
0x1C | NOP          |  absolute,X  | N- Z- C- I- D- V- |   4*   | No Operation
0x3C | NOP          |  absolute,X  | N- Z- C- I- D- V- |   4*   | No Operation
0x5C | NOP          |  absolute,X  | N- Z- C- I- D- V- |   4*   | No Operation
0x7C | NOP          |  absolute,X  | N- Z- C- I- D- V- |   4*   | No Operation
0x9C | invalid      |              |                   |        |
0xBC | LDY oper,X   |  absolute,X  | N+ Z+ C- I- D- V- |   4*   | Load Index Y with Memory
0xDC | NOP          |  absolute,X  | N- Z- C- I- D- V- |   4*   | No Operation
0xFC | NOP          |  absolute,X  | N- Z- C- I- D- V- |   4*   | No Operation
     |              |              |                   |        |
0x1D | ORA oper,X   |  absolute,X  | N+ Z+ C- I- D- V- |   4*   | OR Memory with Accumulator
0x3D | AND oper,X   |  absolute,X  | N+ Z+ C- I- D- V- |   4*   | AND Memory with Accumulator
0x5D | EOR oper,X   |  absolute,X  | N+ Z+ C- I- D- V- |   4*   | Exclusive-OR Memory with Accumulator
0x7D | ADC oper,X   |  absolute,X  | N+ Z+ C+ I- D- V+ |   4*   | Add Memory to Accumulator with Carry
0x9D | STA oper,X   |  absolute,X  | N- Z- C- I- D- V- |   5    | Store Accumulator in Memory
0xBD | LDA oper,X   |  absolute,X  | N+ Z+ C- I- D- V- |   4*   | Load Accumulator with Memory
0xDD | CMP oper,X   |  absolute,X  | N+ Z+ C+ I- D- V- |   4*   | Compare Memory with Accumulator
0xFD | SBC oper,X   |  absolute,X  | N+ Z+ C+ I- D- V+ |   4*   | Subtract Memory from Accumulator with Borrow
     |              |              |                   |        |
0x1E | ASL oper,X   |  absolute,X  | N+ Z+ C+ I- D- V- |   7    | Shift Left One Bit (Memory)
0x3E | ROL oper,X   |  absolute,X  | N+ Z+ C+ I- D- V- |   7    | Rotate One Bit Left (Memory)
0x5E | LSR oper,X   |  absolute,X  | N0 Z+ C+ I- D- V- |   7    | Shift One Bit Right (Memory)
0x7E | ROR oper,X   |  absolute,X  | N+ Z+ C+ I- D- V- |   7    | Rotate One Bit Right (Memory)
0x9E | invalid      |              |                   |        |
0xBE | LDX oper,Y   |  absolute,Y  | N+ Z+ C- I- D- V- |   4*   | Load Index X with Memory
0xDE | DEC oper,X   |  absolute,X  | N+ Z+ C- I- D- V- |   7    | Decrement Memory by One
0xFE | INC oper,X   |  absolute,X  | N+ Z+ C- I- D- V- |   7    | Increment Memory by One
```

### License
[MIT](https://opensource.org/licenses/MIT) - © dtg [at] lengo [dot] org