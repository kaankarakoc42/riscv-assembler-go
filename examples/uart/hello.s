# hello.s — main() that prints "Hello, RV32I!\r\n" once a second.

.global main
.extern uart_puts
.extern delay_ms

.text

main:
    addi sp, sp, -16
    sw   ra, 12(sp)

loop:
    la   a0, msg
    call uart_puts

    li   a0, 1000
    call delay_ms

    j    loop

    lw   ra, 12(sp)
    addi sp, sp, 16
    ret

.data
msg:
    .asciz "Hello, RV32I!\r\n"
