# blink.s — main() that toggles LED bits in a Knight Rider sweep.
#
# Demonstrates: cross-file extern resolution (calls `delay` from start.s),
# memory-mapped IO (LED register at 0x80000000), and pseudo-instructions
# (li, call, ret, mv).

.global main
.extern delay

.text

# Memory-mapped LED register address: 0x80000000.

main:
    addi sp, sp, -16
    sw   ra, 12(sp)

    li   t1, 0x80000000    # t1 ← &LEDs
    li   t2, 0x01          # t2 ← starting LED bit

loop:
    sw   t2, 0(t1)         # write LED pattern

    # delay(0x40000)
    li   a0, 0x40000
    call delay

    # Rotate t2 left by 1 inside the low 6 bits.
    slli t2, t2, 1
    li   t3, 0x40
    bne  t2, t3, no_wrap
    li   t2, 0x01
no_wrap:
    j    loop

    lw   ra, 12(sp)
    addi sp, sp, 16
    ret
