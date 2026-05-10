# RVOB1 — relocatable object file format

A deliberately simple custom binary format. All multi-byte integers are
**little-endian** (matching RV32I memory layout). All offsets in this document
are byte offsets from the start of the file unless noted.

```
   ┌──────────────────────────────────────┐  offset 0
   │            FILE HEADER (24 B)        │
   ├──────────────────────────────────────┤
   │            SECTION TABLE             │  num_sections × variable
   ├──────────────────────────────────────┤
   │            SECTION PAYLOADS          │  raw bytes for each section
   ├──────────────────────────────────────┤
   │            SYMBOL TABLE              │  num_symbols × variable
   ├──────────────────────────────────────┤
   │            RELOCATION TABLE          │  num_relocs × 14 B
   └──────────────────────────────────────┘
```

## File header (fixed 24 bytes)

```
offset  size  field           value / notes
------  ----  --------------- -----------------------------------------
  0      4    magic           "RVOB" = 0x424F5652  (little-endian)
  4      2    version         1
  6      2    flags           reserved, currently 0
  8      4    num_sections    uint32
 12      4    num_symbols     uint32
 16      4    num_relocs      uint32
 20      4    sections_offset uint32  (always 24 in v1)
```

After the header come N section descriptors, then their payloads, then symbols,
then relocations. Offsets to symbol/reloc tables are recoverable by walking
the sections, so we don't store them — keeps the format simple and
hand-editable.

## Section descriptor

```
size  field         notes
----  ------------  -----------------------------------------------
  1   name_len      0..255
  N   name          ASCII bytes (e.g. ".text", ".data")
  4   flags         bit0=executable, bit1=writable, bit2=allocated
  4   size          payload byte count
  4   payload_off   offset (file-relative) to this section's bytes
```

## Section payload

Just `size` raw bytes. For `.text` they are encoded RV32I instructions in
little-endian order. For `.data` they are user-provided bytes.

## Symbol entry

```
size  field         notes
----  ------------  -----------------------------------------------
  1   name_len      0..255
  N   name          ASCII
  1   section_idx   0..num_sections-1, or 0xFF for EXTERN
  1   binding       0=LOCAL, 1=GLOBAL, 2=EXTERN
  4   value         offset within section (or 0 if extern)
```

The `value` field is a *section-relative* offset, not an absolute address.
That's the whole point of a relocatable: the linker decides absolute layout.

## Relocation entry (14 bytes)

```
size  field         notes
----  ------------  -----------------------------------------------
  1   section_idx   which section this patches into
  1   reloc_type    see common/reloc/types.go
  4   offset        byte offset within that section
  4   sym_idx       index into the symbol table
  4   addend        signed; added to the symbol value before patching
```

## Why no string table?

ELF deduplicates strings into a separate `.strtab` section. We don't bother:
inlining names is more readable in a hex editor and a 32-symbol module is a
few hundred bytes either way.

## Why no debug sections?

Out of scope for an educational toolchain. A future `RVOB2` could add them.

## Hex-editor walkthrough

For a tiny program

```asm
.global main
.text
main:
    addi x1, x0, 5
    ret             # alias for jalr x0, 0(x1)
```

the produced `.ro` file looks like:

```
00000000  52 56 4F 42 01 00 00 00  01 00 00 00 01 00 00 00  RVOB............
00000010  00 00 00 00 18 00 00 00  05 2E 74 65 78 74 01 00  ......text......
00000020  00 00 08 00 00 00 38 00  00 00 93 00 50 00 67 80  ......8.....P.g.
00000030  00 00 04 6D 61 69 6E 00  01 00 00 00 00            ..main.......
```

Header = 24 bytes, one section descriptor (.text, 8 bytes payload), the
8 bytes of payload (encoded `addi x1,x0,5` and `jalr x0,0(x1)`), and a
single symbol "main" (binding=GLOBAL=1, section=0, value=0).

That's the whole file. No symbol-table strings shared, no segments, no
relocations (this trivial program has nothing to relocate).
