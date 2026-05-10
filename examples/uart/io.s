# io.s — provides the ms-tick reader used by uart.s/delay_ms.

.global read_ms

.text

read_ms:
    li   t0, 0x80000020
    lw   a0, 0(t0)
    ret
