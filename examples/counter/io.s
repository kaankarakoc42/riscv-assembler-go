# io.s — small IO helpers (used by main in counter.s).
#
# Provides:
#   set_leds(uint32_t value)        — writes value to LED register
#   read_ms(void) -> a0             — returns current ms-tick counter

.global set_leds
.global read_ms

.text

# void set_leds(uint32_t v);
#   v is in a0 by RISC-V calling convention.
set_leds:
    li   t0, 0x80000000   # &LEDs
    sw   a0, 0(t0)
    ret

# uint32_t read_ms(void);
read_ms:
    li   t0, 0x80000020   # &ms_tick
    lw   a0, 0(t0)
    ret
