# start.s — reset entry point and a busy-wait delay routine.
# Both `_start` and `delay` are GLOBAL; main() in blink.s calls them.

.global _start
.global delay
.extern main

.text

# ─── _start ─────────────────────────────────────────────────────────────
# PicoRV32 jumps here on reset (PC = 0x00000000). We set up a stack at
# the top of BRAM and then tail-call into the C-style main().
_start:
    li   sp, 0x2000        # sp = top of 8 KiB BRAM
    call main
hang:
    j    hang              # if main returns, hang.

# ─── delay ──────────────────────────────────────────────────────────────
# Busy-wait the number of "iterations" passed in a0. ~3 cycles per loop.
delay:
    mv   t0, a0            # t0 = counter
delay_loop:
    addi t0, t0, -1
    bnez t0, delay_loop
    ret
