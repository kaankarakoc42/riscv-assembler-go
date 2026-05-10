# blink.s — main() that toggles LED bits in a Knight Rider sweep.
#
# Demonstrates: cross-file extern resolution (calls `delay` from start.s),
# memory-mapped IO (LED register at 0x40000000 in our minimal SoC), and
# pseudo-instructions (li, call, ret, mv).
#
# NOTE on the LED address.
#
# The companion FPGA top-level (fpga/soc_top.v + bram.v) uses 0x80000000
# for its LED register because the high bit nicely separates RAM from MMIO.
# However, the simpler educational top-level (Tang Nano 9K, single-BRAM
# tap_module.v) uses 0x40000000. We follow that convention here so the
# demo runs unchanged on either SoC.

.global main
.extern delay

.text

# Memory-mapped LED register address: 0x40000000.

main:
    addi sp, sp, -16
    sw   ra, 12(sp)

    li   t1, 0x40000000    # t1 ← &LEDs
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
