# RV32I instruction encoding cheat sheet

All instructions are 32 bits, little-endian in memory.

```
   31           25 24       20 19       15 14   12 11        7 6      0
R: ┌─────────────┬───────────┬───────────┬───────┬───────────┬───────┐
   │   funct7    │    rs2    │    rs1    │funct3 │     rd    │opcode │
   ├─────────────┴───────────┼───────────┼───────┼───────────┼───────┤
I: │      imm[11:0]          │    rs1    │funct3 │     rd    │opcode │
   ├─────────────┬───────────┼───────────┼───────┼───────────┼───────┤
S: │  imm[11:5]  │    rs2    │    rs1    │funct3 │ imm[4:0]  │opcode │
   ├─┬───────────┼───────────┼───────────┼───────┼───────────┼─┬─────┤
B: │ │imm[10:5]  │    rs2    │    rs1    │funct3 │ imm[4:1]  │ │opcode│
   │↑12                                                       ↑11    │
   ├─┴───────────┴───────────┴───────────┴───────┴───────────┴─┴─────┤
U: │              imm[31:12]                     │     rd    │opcode │
   ├─┬─────────────────────┬─┬───────────────────┬───────────┬───────┤
J: │ │   imm[10:1]         │ │   imm[19:12]      │     rd    │opcode │
   │↑20                    ↑11                                       │
   └─────────────────────────────────────────────────────────────────┘
```

## The B/J scrambles, explained

Why is `imm[11]` in bit 7 of a B-type? The RISC-V designers fix `imm[0]=0` for
all branches and jumps (2-byte alignment), so they only need to encode 12
significant bits of B-immediate and 20 significant bits of J-immediate.
By placing `rs1`/`rs2` in *the same bit positions* across R/I/S/B, register
fetch can begin **before** the rest of the instruction has been decoded —
register-file reads become a fixed wire trace from the instruction word.

The cost: the immediate is non-contiguous. We pay for it once in software.

```
B-type (13-bit signed branch offset, imm[0]=0):
   imm[12]   = inst[31]
   imm[11]   = inst[7]
   imm[10:5] = inst[30:25]
   imm[4:1]  = inst[11:8]

J-type (21-bit signed jump offset, imm[0]=0):
   imm[20]    = inst[31]
   imm[19:12] = inst[19:12]
   imm[11]    = inst[20]
   imm[10:1]  = inst[30:21]
```

## Worked examples

| Assembly                | Hex word     | Why                                                       |
|-------------------------|--------------|-----------------------------------------------------------|
| `add x1, x2, x3`        | `0x003100B3` | `funct7=0 rs2=3 rs1=2 f3=0 rd=1 op=0x33`                  |
| `sub x5, x6, x7`        | `0x407302B3` | `funct7=0x20`                                             |
| `addi x1, x0, 5`        | `0x00500093` | `imm=5 rs1=0 f3=0 rd=1 op=0x13`                           |
| `addi x5, x6, -1`       | `0xFFF30293` | `imm=0xFFF (12-bit 2s comp)`                              |
| `sw x5, 8(x2)`          | `0x00512423` | `imm=8 split as 0/8`                                      |
| `beq x1, x2, +8`        | `0x00208463` | `imm[12]=0 [11]=0 [10:5]=0 [4:1]=4`                       |
| `bne x5, x6, -4`        | `0xFE629EE3` | `imm[12]=1 [11]=1 [10:5]=0x3F [4:1]=0xE`                  |
| `lui x1, 0x12345`       | `0x123450B7` | upper 20 bits + rd=1 + op=0x37                            |
| `jal x1, +16`           | `0x010000EF` | `imm[19:12]=0 [11]=0 [10:1]=8 [20]=0`                     |
| `jal x0, -8`            | `0xFF9FF06F` | `imm[20]=1 [19:12]=0xFF [11]=1 [10:1]=0x3FC`              |
| `slli x1, x2, 5`        | `0x00511093` | `funct7=0 shamt=5 rs1=2 f3=1 rd=1 op=0x13`                |
| `ecall`                 | `0x00000073` |                                                           |
| `ebreak`                | `0x00100073` |                                                           |

The same hand-derivations are pinned in `common/isa/encoding_test.go` — if any
of them ever fails, the rest of the toolchain is wrong.
