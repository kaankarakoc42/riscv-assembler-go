# FPGA integration: PicoRV32 on Tang Nano 9K

## What you need

* **Tang Nano 9K** (or compatible board with a Gowin GW1NR-9 / GW1NR-LV9 FPGA).
* **Gowin EDA** (free for the GW1N family).
* **PicoRV32 core** — fetch `picorv32.v` from
  https://github.com/YosysHQ/picorv32 and add it to your project.

## Wiring overview

```
                   ┌─────────────────────────────────────────────┐
                   │                soc_top.v                    │
                   │                                             │
       clk ───────►│  resetn synchroniser                        │
       rst_n──────►│                                             │
                   │  ┌──────────┐    mem_valid       ┌────────┐ │
                   │  │ picorv32 │◄──────────────────►│ decode │ │
                   │  └─┬────────┘                    └─┬──────┘ │
                   │    │                               │        │
                   │    │ mem_addr                      │        │
                   │    ▼                               ▼        │
                   │  ┌──────────┐         ┌─────┐ ┌─────────┐   │
                   │  │   bram   │         │ LED │ │ UART TX │   │
                   │  │  8 KiB   │         │ reg │ │  bitbang│   │
                   │  │$readmemh │         └─────┘ └─────────┘   │
                   │  └──────────┘                               │
                   └─────────────────────────────────────────────┘
                                                      │   │
                                                  leds[5:0] uart_tx
```

## Memory map

| Address       | Width | Direction | Function                              |
|---------------|-------|-----------|---------------------------------------|
| `0x00000000`+ | 32    | R/W       | BRAM (instruction + data, 8 KiB)      |
| `0x80000000`  | 32    | W         | LED register, low 6 bits drive LEDs   |
| `0x80000010`  | 32    | W/R       | UART TX data; read returns busy=bit0  |
| `0x80000020`  | 32    | R         | Free-running ms tick counter          |

## How `program.mem` is loaded

The BRAM module declares:

```verilog
initial $readmemh(INIT_FILE, mem);
```

Both Gowin's synthesiser and `iverilog` simulation evaluate `$readmemh`
at *elaboration time*, baking the contents into the BRAM's initial
configuration. **The file must sit next to the synthesised top module**
(or be referenced with an absolute path).

## Reset semantics

PicoRV32's reset vector is `0x00000000`. Our BRAM starts at the same
address, so on the first cycle after `resetn` deasserts the CPU fetches
word 0 from the BRAM, which is the first instruction of `_start` (in your
`start.s`).

Make sure your linker script's `text_at = rom` so that `.text` lives at
exactly `0x00000000`.

## Clocking assumptions

* The Tang Nano 9K's onboard oscillator runs at **27 MHz**. We pass that
  directly to PicoRV32 — no PLL needed for our demo programs.
* If you want a different clock rate, instantiate a Gowin PLL (`Gowin_rPLL`)
  and feed `soc_top.clk` from it. Update the `CLK_HZ` parameter of
  `soc_top` to match — the UART divider and ms-tick counter use it.

## Building

```sh
# 1. Assemble and link the demo
./rvasm -o build/start.ro examples/blink/start.s
./rvasm -o build/blink.ro examples/blink/blink.s
./rvld  -script examples/blink/link.toml \
        -o build/blink \
        build/start.ro build/blink.ro

# 2. Copy build/blink.mem next to soc_top.v
cp build/blink.mem fpga/program.mem

# 3. Open Gowin EDA, add:
#      fpga/soc_top.v
#      fpga/bram.v
#      <picorv32.v from upstream>
#      fpga/tang_nano_9k.cst
#    Set top = soc_top, set device = GW1NR-LV9QN88PC6/I5 (Nano 9K).
#    Run "Synthesize" → "Place & Route" → "Program Device".
```

## Bringing it up on hardware

1. Plug the Tang Nano 9K into USB.
2. In Gowin Programmer, click "Program SRAM" (volatile, fast) or
   "Program Flash" (persistent).
3. The 6 onboard LEDs should start blinking per `examples/blink`.
4. Open a serial terminal at 115200-8-N-1 on the BL616 USB-UART bridge.
   For the UART demo you'll see the message stream once per second.

## Troubleshooting

* **All LEDs stay on.** They're active-low; `led_reg = 0` lights them.
  Write something nonzero to `0x80000000`.
* **Constant timeout / hang.** Check that `program.mem` is in the
  synthesis directory and that its first line is a valid 32-bit hex word
  (no `0x` prefix, exactly 8 hex digits).
* **Garbled UART.** Verify `CLK_HZ` matches the actual clock rate; it
  drives the baud divider.
