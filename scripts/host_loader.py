#!/usr/bin/env python3
"""Send a program image to the FPGA UART software loader."""

from __future__ import annotations

import argparse
import pathlib
import struct
import sys
import time


MAGIC = 0x55
CMD_WRITE = 1
CMD_START = 2


def read_bin(path: pathlib.Path) -> bytes:
    data = path.read_bytes()
    return data + (b"\x00" * ((4 - len(data) % 4) % 4))


def read_mem(path: pathlib.Path) -> bytes:
    out = bytearray()
    for line_no, raw in enumerate(path.read_text().splitlines(), 1):
        line = raw.split("#", 1)[0].split("//", 1)[0].strip()
        if not line:
            continue
        if line.startswith("@"):
            raise ValueError(f"{path}:{line_no}: @address records are not supported")
        out += struct.pack("<I", int(line, 16) & 0xFFFFFFFF)
    return bytes(out)


def image_bytes(path: pathlib.Path, fmt: str) -> bytes:
    if fmt == "auto":
        suffix = path.suffix.lower()
        fmt = "mem" if suffix in {".mem", ".hex"} else "bin"
    if fmt == "bin":
        return read_bin(path)
    if fmt == "mem":
        return read_mem(path)
    raise ValueError(f"unknown format: {fmt}")


def checksum(payload: bytes) -> int:
    # loader.s protokolüne göre: low 8 bits of cmd + addr bytes + words + data bytes
    # MAGIC (0x55) değeri kesinlikle checksum toplamına dahil EDİLMEMELİDİR!
    return sum(payload) & 0xFF

def write_frame(port, cmd: int, addr: int, data: bytes, words: int) -> str:
    # 1. Başlık alanını oluştur: [cmd (1B)] + [addr (4B, Little-Endian)] + [words (1B)]
    header = bytes([cmd]) + struct.pack("<I", addr) + bytes([words])
    
    # 2. Checksum hesapla: Sadece cmd, adres, kelime sayısı ve verinin toplamı
    payload_to_check = header + data
    cs = checksum(payload_to_check)
    
    # 3. Nihai paketi birleştir: [MAGIC (0x55)] + [Header] + [Data] + [Checksum (1B)]
    frame = bytes([MAGIC]) + payload_to_check + bytes([cs])
    
    # Karta gönder
    port.write(frame)
    port.flush()
    
    # Karttan yanıt bekle ('K' = Başarılı, 'E' = Hata, 'S' = Başlatıldı)
    reply = port.read(1)
    if not reply:
        raise TimeoutError("no reply from loader")
    return reply.decode("ascii", errors="replace")

def wait_ready(port, timeout: float) -> bool:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if port.read(1) == b"R":
            return True
    return False


def send_image(args: argparse.Namespace) -> None:
    try:
        import serial
    except ImportError:  # pragma: no cover - user environment guard
        print("pyserial is required: python -m pip install pyserial", file=sys.stderr)
        raise SystemExit(2)

    path = pathlib.Path(args.image)
    data = image_bytes(path, args.format)
    if args.entry is None:
        args.entry = args.base

    with serial.Serial(args.port, args.baud, timeout=args.timeout) as port:
        if args.reset_wait:
            if not wait_ready(port, args.reset_wait):
                print("warning: loader ready byte was not seen; sending anyway", file=sys.stderr)

        total_words = len(data) // 4
        sent_words = 0
        for off in range(0, len(data), args.chunk_words * 4):
            chunk = data[off : off + args.chunk_words * 4]
            words = len(chunk) // 4
            reply = write_frame(port, CMD_WRITE, args.base + off, chunk, words)
            if reply != "K":
                raise RuntimeError(f"loader rejected write at 0x{args.base + off:08x}: {reply!r}")
            sent_words += words
            if args.verbose:
                print(f"wrote {sent_words}/{total_words} words", file=sys.stderr)

        reply = write_frame(port, CMD_START, args.entry, b"", 0)
        if reply != "S":
            raise RuntimeError(f"loader rejected start at 0x{args.entry:08x}: {reply!r}")

    print(f"loaded {len(data)} bytes to 0x{args.base:08x}, started 0x{args.entry:08x}")


def parse_args(argv: list[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("image", help="program image, usually build/<program>.bin or .mem")
    p.add_argument("-p", "--port", required=True, help="serial port, e.g. COM5 or /dev/ttyUSB0")
    p.add_argument("-b", "--baud", type=int, default=115200)
    p.add_argument("--base", type=lambda s: int(s, 0), default=0x400)
    p.add_argument("--entry", type=lambda s: int(s, 0))
    p.add_argument("--format", choices=["auto", "bin", "mem"], default="auto")
    p.add_argument("--chunk-words", type=int, default=16, choices=range(1, 256), metavar="1..255")
    p.add_argument("--timeout", type=float, default=1.0)
    p.add_argument("--reset-wait", type=float, default=3.0, help="seconds to wait for loader 'R'")
    p.add_argument("-v", "--verbose", action="store_true")
    return p.parse_args(argv)


def main(argv: list[str]) -> int:
    try:
        send_image(parse_args(argv))
        return 0
    except Exception as exc:
        print(f"host_loader: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
