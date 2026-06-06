# UART software loader for Tang Nano 9K PicoRV32 SoC.
#
# Protocol, 115200 8N1:
#   frame  = 0x55, cmd, addr[0], addr[1], addr[2], addr[3], words, data..., sum
#   sum    = low 8 bits of cmd + addr bytes + words + data bytes
#   cmd=1  WRITE: data contains "words" little-endian 32-bit words, writes at addr
#   cmd=2  START: no data, jumps to addr after checksum passes
#
# Replies:
#   'R' ready after reset, 'K' write ok, 'E' checksum/command error, 'S' starting.

.global _start

.text
_start:
    li   sp, 0x2000
    li   s0, 0x8000000c
    li   s1, 0x80000008
    li   s2, 0x80000014
    li   s3, 0x80000010

    li   a0, 82
    call putc

wait_magic:
    call getc
    li   t0, 0x55
    bne  a0, t0, wait_magic

    call getc
    mv   s4, a0
    mv   s7, a0

    call getc
    mv   s5, a0
    add  s7, s7, a0

    call getc
    slli t0, a0, 8
    or   s5, s5, t0
    add  s7, s7, a0

    call getc
    slli t0, a0, 16
    or   s5, s5, t0
    add  s7, s7, a0

    call getc
    slli t0, a0, 24
    or   s5, s5, t0
    add  s7, s7, a0

    call getc
    mv   s6, a0
    add  s7, s7, a0

    li   t0, 1
    beq  s4, t0, write_words
    li   t0, 2
    beq  s4, t0, check_start
    j    bad_frame

write_words:
    beqz s6, check_write

    call getc
    mv   t1, a0
    add  s7, s7, a0

    call getc
    slli t2, a0, 8
    or   t1, t1, t2
    add  s7, s7, a0

    call getc
    slli t2, a0, 16
    or   t1, t1, t2
    add  s7, s7, a0

    call getc
    slli t2, a0, 24
    or   t1, t1, t2
    add  s7, s7, a0

    sw   t1, 0(s5)
    addi s5, s5, 4
    addi s6, s6, -1
    j    write_words

check_write:
    call getc
    andi s7, s7, 255
    bne  a0, s7, bad_frame
    li   a0, 75
    call putc
    j    wait_magic

check_start:
    call getc
    andi s7, s7, 255
    bne  a0, s7, bad_frame
    li   a0, 83
    call putc
    jr   s5

bad_frame:
    li   a0, 69
    call putc
    j    wait_magic

getc:
    lw   t0, 0(s0)
    andi t0, t0, 1
    beqz t0, getc
    lw   a0, 0(s1)
    andi a0, a0, 255
    ret

putc:
    lw   t0, 0(s2)
    andi t0, t0, 1
    bnez t0, putc
    sw   a0, 0(s3)
    ret
