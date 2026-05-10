# rvtoolchain — an educational RV32I assembler + linker for PicoRV32

A from-scratch, fully open RV32I toolchain written in Go. Designed for
university computer-architecture courses and small FPGA SoCs (PicoRV32 +
Tang Nano 9K).

## What you get

| Tool       | Purpose                                                 |
|------------|---------------------------------------------------------|
| `rvasm`    | Assemble one `.s` file into a relocatable object (`.ro`)|
| `rvld`     | Link multiple `.ro` files into a flat memory image      |
| `rvdump`   | Inspect object files: symbols, relocations, disassembly |

Produces three flavors of output:

* `program.bin`  — raw little-endian flat image
* `program.hex`  — Intel HEX (loadable by most flashers)
* `program.mem`  — `$readmemh` BRAM init (32-bit words, one per line)

## Quick start

```sh
# Build the toolchain
go build ./cmd/rvasm
go build ./cmd/rvld
go build ./cmd/rvdump

# Assemble two source files
./rvasm -o build/start.ro examples/blink/start.s
./rvasm -o build/blink.ro examples/blink/blink.s

# Link them
./rvld -script examples/blink/link.toml \
       -o    build/blink \
       build/start.ro build/blink.ro

# Inspect
./rvdump build/blink.ro
```

## Project layout

```
.
├── cmd/                  # Executable entry points
│   ├── rvasm/            #   assembler
│   ├── rvld/             #   linker
│   └── rvdump/           #   object inspector
├── common/               # Shared, language-agnostic code
│   ├── isa/              #   RV32I encoding tables, registers
│   ├── obj/              #   RVOB1 object format (read + write)
│   └── reloc/            #   relocation type definitions
├── assembler/            # rvasm internals
│   ├── lexer/            #   token stream
│   ├── parser/           #   directives + instructions → AST
│   └── encoder/          #   AST → bytes + symbols + relocs
├── linker/               # rvld internals
│   ├── script/           #   linker-script parser
│   ├── symtab/           #   global symbol resolution
│   ├── reloc/            #   relocation application
│   └── image/            #   flat-image builder + hex/bram emitters
├── examples/             # Demo programs (multi-file)
│   ├── blink/
│   ├── counter/
│   └── uart/
├── fpga/                 # Verilog: BRAM + PicoRV32 SoC
└── docs/                 # Architecture & format documentation
```

## Documentation

* [`docs/architecture.md`](docs/architecture.md) — pipeline & data flow
* [`docs/object-format.md`](docs/object-format.md) — RVOB1 binary layout
* [`docs/encoding.md`](docs/encoding.md) — RV32I instruction encoding cheat sheet
* [`docs/relocation.md`](docs/relocation.md) — bit-level relocation math
* [`docs/fpga.md`](docs/fpga.md) — wiring it into PicoRV32

## Status

| Phase | Component                         | Status |
|-------|-----------------------------------|--------|
| 1     | Instruction encoder               | done   |
| 2     | Lexer + parser + symbol table     | done   |
| 3     | Object file format                | done   |
| 4     | Linker core                       | done   |
| 5     | Relocation engine                 | done   |
| 6     | HEX / BRAM emitters               | done   |
| 7     | FPGA integration                  | done   |
| 8     | Tests + demos                     | done   |

## License

MIT. Use freely in coursework.
