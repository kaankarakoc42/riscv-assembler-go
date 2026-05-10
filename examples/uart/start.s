# start.s — reset entry.

.global _start
.extern main

.text
_start:
    li   sp, 0x2000
    call main
hang:
    j    hang
