# uart.s — UART transmit primitives.
#
# Provides:
#   uart_putc(c)          — block until UART is free, then send 1 byte
#   uart_puts(ptr)        — send NUL-terminated string
#   delay_ms(n)           — busy-wait approx n milliseconds
#
# UART register: 0x80000010
#   write   any byte → enqueues for transmission
#   read    bit0 = busy

.global uart_putc
.global uart_puts
.global delay_ms

.extern read_ms       # provided by io.s

.text

# void uart_putc(char c);
# Spins on the busy bit, then writes the byte.
uart_putc:
    li   t0, 0x80000010
putc_wait:
    lw   t1, 0(t0)
    andi t1, t1, 1
    bnez t1, putc_wait
    sw   a0, 0(t0)
    ret

# void uart_puts(const char* s);
# Walks bytes from a0 until a NUL.
uart_puts:
    addi sp, sp, -16
    sw   ra, 12(sp)
    sw   s0, 8(sp)

    mv   s0, a0           # s0 = pointer
puts_loop:
    lbu  a0, 0(s0)
    beqz a0, puts_done
    call uart_putc
    addi s0, s0, 1
    j    puts_loop
puts_done:
    lw   s0, 8(sp)
    lw   ra, 12(sp)
    addi sp, sp, 16
    ret

# void delay_ms(uint32_t n);
# Reads the ms-tick counter and waits until n have elapsed.
delay_ms:
    addi sp, sp, -16
    sw   ra, 12(sp)
    sw   s0, 8(sp)
    sw   s1, 4(sp)

    mv   s0, a0           # s0 = n
    call read_ms
    mv   s1, a0           # s1 = start time
delay_spin:
    call read_ms
    sub  a0, a0, s1
    blt  a0, s0, delay_spin

    lw   s1, 4(sp)
    lw   s0, 8(sp)
    lw   ra, 12(sp)
    addi sp, sp, 16
    ret
