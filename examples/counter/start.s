# start.s — common reset entry, mirrored from blink for clarity.

.global _start
.extern main

.text
_start:
    li   sp, 0x2000
    call main
hang:
    j    hang
