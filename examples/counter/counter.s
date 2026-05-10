# counter.s — main(): displays the low 6 bits of the ms-tick counter on LEDs.
#
# Calls into io.s for the actual register accesses, exercising the linker's
# cross-module symbol resolution.

.global main
.extern set_leds
.extern read_ms

.text

main:
    # Save ra
    addi sp, sp, -16
    sw   ra, 12(sp)

loop:
    call read_ms        # a0 ← current ms count
    # Mask to low 6 bits so it fits the LED count
    andi a0, a0, 0x3F
    call set_leds
    j    loop

    lw   ra, 12(sp)
    addi sp, sp, 16
    ret
