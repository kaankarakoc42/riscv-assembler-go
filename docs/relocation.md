# Relocation engine

A relocation answers the question: *"What value should I splice into the
machine word at file/section offset O?"*

Every relocation has four ingredients:

* **S** — the final absolute address of the symbol it references.
* **A** — the addend (constant added to S before patching). 0 in nearly all
  our cases, supported for psABI compatibility.
* **P** — the absolute address of the patch site (the instruction word
  itself), used by PC-relative kinds.
* **The instruction word at offset O** — must be **OR-ed in**, not
  overwritten, because the assembler may have left the register/opcode
  fields populated with placeholder zero immediates already encoded in the
  word.

The general flow per relocation:

```text
1. Load the 32-bit word W from image[P].
2. Compute the target value V according to the reloc type
   (e.g. V = S + A   for absolute,
         V = S + A - P  for pc-relative).
3. Mask out the immediate bits in W.
4. OR in the encoded form of V (using the same scrambled bit positions used
   by the corresponding instruction format).
5. Write W back to image[P].
6. Range-check V; reject if it doesn't fit in the destination field.
```

## Types we implement

| Mnemonic example       | Reloc                       | What gets patched                            |
|------------------------|-----------------------------|----------------------------------------------|
| `.word symbol`         | `R_RV32_32`                 | All 32 bits at the patch site                |
| `beq x1,x2,target`     | `R_RV32_BRANCH`             | B-type imm[12,10:5,4:1,11]                   |
| `jal x1,target`        | `R_RV32_JAL`                | J-type imm[20,10:1,11,19:12]                 |
| `lui rd, %hi(sym)`     | `R_RV32_HI20`               | Upper 20 bits, with HI/LO carry              |
| `addi rd,rd,%lo(sym)`  | `R_RV32_LO12_I`             | I-type imm[11:0]                             |
| `sw rs,%lo(sym)(rd)`   | `R_RV32_LO12_S`             | S-type imm[11:5,4:0]                         |
| `auipc rd, %pcrel_hi`  | `R_RV32_PCREL_HI20`         | Upper 20 bits, PC-relative                   |
| `addi/lw + %pcrel_lo`  | `R_RV32_PCREL_LO12_I`       | LO12 of (S - P_paired_auipc)                 |

## HI20 / LO12 sign-extension carry

The LO12 immediate is **sign-extended** when added by `addi`. For an absolute
target whose bit 11 is set, we must add 1 to HI20 so the addition still
produces the correct full address.

Worked example: load address 0x1000_0FFF.

```
LO12 = 0x0FFF (= -1 when sign-extended, since bit 11 is 1)
HI20 = 0x10001  (round up because LO12 will subtract)
final = (HI20 << 12) + sign_extend(LO12)
      = 0x10001000 + 0xFFFFFFFF      ; -1
      = 0x10000FFF                   ; ✓
```

## Branch encoding deep-dive

Computing the patched word for `beq x1,x2,+8` at PC=0:

```
V = +8 (target=8, source PC=0, no addend)
imm[12]   = (V >> 12) & 1 = 0
imm[11]   = (V >> 11) & 1 = 0
imm[10:5] = (V >> 5)  & 0x3F = 0
imm[4:1]  = (V >> 1)  & 0xF  = 4

inst[31]    = imm[12]
inst[30:25] = imm[10:5]
inst[7]     = imm[11]
inst[11:8]  = imm[4:1]

mask = inst & ~B-imm-mask
     = 0x01FFF07F
patched = mask | (imm[12]<<31) | (imm[10:5]<<25) | (imm[4:1]<<8) | (imm[11]<<7)
```

## JAL encoding deep-dive

`jal x1,+16`:

```
V = +16
imm[20]    = 0
imm[19:12] = 0
imm[11]    = 0
imm[10:1]  = 8

inst[31]    = imm[20]
inst[19:12] = imm[19:12]
inst[20]    = imm[11]
inst[30:21] = imm[10:1]
```

## PCREL HI/LO pairing

Our convention: `R_RV32_PCREL_LO12_I` carries **the same symbol** as the
paired `R_RV32_PCREL_HI20`. The linker:

1. Computes V_hi = (S - P_auipc) split into HI20/LO12 with the carry.
2. For the auipc site: patches HI20.
3. For the addi site: patches LO12 using the *same* split, NOT
   recomputed against the addi's own PC. This matches GNU as behavior.

Sequencing: the assembler emits `auipc` immediately followed by `addi`, so
they live in the same input section at offsets `pc` and `pc+4`. Our
relocator finds the matching auipc by scanning relocations from the same
section that share the same SymIdx; for our simple toolchain the previous
PCREL_HI20 entry (smaller offset, same section, same sym) is the partner.

## Range checks

| Type            | Field width | Range                    |
|-----------------|-------------|--------------------------|
| R_RV32_32       | 32          | full                     |
| R_RV32_BRANCH   | 13 signed   | ±4096, even              |
| R_RV32_JAL      | 21 signed   | ±1 MiB, even             |
| R_RV32_HI20     | 32          | full                     |
| R_RV32_LO12_*   | 12 signed   | ±2048                    |
| R_RV32_PCREL_*  | as above    | computed against P       |

A relocation that overflows its field is a hard error; no truncation, no
silent wrap.
