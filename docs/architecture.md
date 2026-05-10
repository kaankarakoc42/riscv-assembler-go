# Architecture

```
┌──────────────────┐                ┌──────────────────┐                ┌──────────────────┐
│   .s source      │   rvasm        │  .ro object      │   rvld         │  flat image +    │
│   (RV32I asm)    │ ─────────────► │  (RVOB1 binary)  │ ─────────────► │  .hex + .mem     │
│   directives     │                │  text/data/      │                │  for FPGA BRAM   │
│   labels         │                │  symtab/relocs   │                │                  │
└──────────────────┘                └──────────────────┘                └──────────────────┘
        │                                   │                                    │
        │                                   │                                    │
   lexer ─► parser ─► encoder         reader/writer in obj                hex / mem emitters
                                            │
                                            ▼
                                     resolution, relocation
```

## Stage breakdown

### 1. Lexing (`assembler/lexer`)

Plain hand-written scanner. Tokens:

* `IDENT` — `addi`, `x5`, `main`, `loop_top`
* `NUMBER` — `123`, `0xff`, `0b1010`, `-1`
* `STRING` — `"hello\n"`
* `COMMA`, `LPAREN`, `RPAREN`, `COLON`
* `DIRECTIVE` — `.text`, `.data`, `.global`, `.word`, …
* `EOL`, `EOF`

Comments start at `#` or `;` and run to end of line. Whitespace insignificant
inside lines but newlines matter (one statement per line).

### 2. Parsing (`assembler/parser`)

A line-based recursive-descent parser that produces an `AST`:

* `LabelStmt` — `name:`
* `DirectiveStmt` — `.word 0x1234`, `.global main`
* `InstrStmt` — `addi x1, x2, 12`

The parser is permissive about register naming (`x5`, `t0`, `s1`) — it normalizes
to numeric register IDs in `common/isa/registers.go`.

### 3. Encoding (`assembler/encoder`)

Two-pass over the AST:

* **Pass 1** — assign offsets to every instruction and `.data` byte. Record
  labels in the local symbol table. Generate placeholder relocations for any
  expression that refers to a symbol whose final address isn't known.
* **Pass 2** — emit machine bytes for instructions whose immediates are fully
  resolved at assembly time (purely numeric).

For symbol-referencing operands we **always** emit a relocation, even if the
symbol is local — the linker re-bases sections, so the absolute or PC-relative
math must be redone there.

### 4. Object writing (`common/obj`)

The encoder hands the writer a `Module`:

```
Module
├── Sections      [.text, .data]
├── Symbols       (name, section, offset, binding)
└── Relocations   (section, offset, sym index, type, addend)
```

Writer marshals to **RVOB1** (see [object-format.md](object-format.md)).

### 5. Linking (`linker`)

* Parse linker script (`linker/script`) → memory layout.
* Load every `.ro` (`common/obj` reader) into in-memory `Module`s.
* **Section merge**: concatenate all `.text` sections in input order, then all
  `.data`. Each input section gets a `linkOffset` recorded so we can translate
  per-module symbol values to global addresses.
* **Global symbol table** (`linker/symtab`):
  * Local symbols stay private to their module (kept for relocations).
  * Global symbols enter a global map; duplicates are an error.
  * External symbols are resolved against the global map; unresolved → error.
* **Relocation pass** (`linker/reloc`): walk every relocation, compute the
  patched immediate using the final symbol address and the relocation site's
  final PC, splice it into the instruction word.
* **Image emit** (`linker/image`):
  * `.bin` — raw bytes
  * `.hex` — Intel HEX records
  * `.mem` — one 32-bit hex word per line, `$readmemh` compatible

## Why a custom object format?

ELF is well-documented but visually noisy: sections, segments, dynamic tables,
program headers, string tables… Students should be able to `xxd` a `.ro` file
and identify every byte. RVOB1 is roughly **80 bytes of header + payload**.

## Why explicit relocation types?

Each RV32I immediate has a unique bit-permutation. We model **each variant** as
its own relocation type so the patching code is a tiny bit-twiddle per type
rather than a giant switch. See [relocation.md](relocation.md).
