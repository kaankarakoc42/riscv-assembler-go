// tap_module.v — minimal PicoRV32 SoC for Tang Nano 9K (single-BRAM variant).
//
// Drop-in compatible with the file you had — same module name, same port
// list. Two important fixes over the original:
//
//   1. RAM is only matched when mem_addr lies inside MEM_BYTES. Unmapped
//      writes (e.g. to an LED MMIO range that doesn't match) used to fall
//      through into RAM and corrupt program memory. Now they're ignored.
//
//   2. resetn is synchronised against `clk` to avoid metastability when the
//      user mashes the S1 button.
//
// MEMORY MAP
//   0x00000000 .. (MEM_BYTES-1)   : BRAM (text + data), $readmemh-loaded
//   0x40000000                    : LED register (write-only, 6 bits)
//
// LED polarity
//   Tang Nano 9K LEDs are active-low (lit when output = 0). We invert in
//   hardware so software writes a 1 to mean "lit".
//
// MEMORY SIZE
//   Default 4 KiB (1024 × 32-bit words). Change MEM_WORDS_LOG2 to grow.
//   Update your linker script's rom_size to match (4 KiB = 0x1000,
//   8 KiB = 0x2000).

`default_nettype none

module top_module #(
    parameter integer MEM_WORDS_LOG2 = 10  // 2^10 words = 4 KiB
) (
    input  wire        clk,
    input  wire        resetn,
    output reg  [5:0]  leds = 6'b111111
);
    localparam integer MEM_WORDS = (1 << MEM_WORDS_LOG2);
    localparam integer MEM_BYTES = MEM_WORDS * 4;

    // ─── Reset synchroniser ────────────────────────────────────────────
    reg [2:0] rst_sync = 3'b000;
    wire      resetn_sync = rst_sync[2];
    always @(posedge clk) rst_sync <= {rst_sync[1:0], resetn};

    // ─── PicoRV32 native memory bus ────────────────────────────────────
    wire        mem_valid;
    wire        mem_instr;
    reg         mem_ready;
    wire [31:0] mem_addr;
    wire [31:0] mem_wdata;
    wire [3:0]  mem_wstrb;
    reg  [31:0] mem_rdata;

    picorv32 cpu (
        .clk      (clk),
        .resetn   (resetn_sync),
        .mem_valid(mem_valid),
        .mem_instr(mem_instr),
        .mem_ready(mem_ready),
        .mem_addr (mem_addr),
        .mem_wdata(mem_wdata),
        .mem_wstrb(mem_wstrb),
        .mem_rdata(mem_rdata)
    );

    // ─── Address decoding ──────────────────────────────────────────────
    // RAM occupies addresses 0..MEM_BYTES-1. Anything with bit 31:24 !=
    // 0x00 is treated as IO (or ignored).
    wire ram_sel = (mem_addr < MEM_BYTES);
    wire led_sel = (mem_addr == 32'h40000000);

    // ─── BRAM ──────────────────────────────────────────────────────────
    reg [31:0] memory [0:MEM_WORDS-1];

    initial begin
        $readmemh("program.hex", memory);
    end

    wire [MEM_WORDS_LOG2-1:0] word_addr = mem_addr[MEM_WORDS_LOG2+1:2];

    // Single-cycle ack. Writes happen the same cycle valid is high.
    always @(posedge clk) begin
        mem_ready <= 1'b0;

        if (mem_valid && !mem_ready) begin
            mem_ready <= 1'b1;

            if (|mem_wstrb) begin
                // Write path
                if (ram_sel) begin
                    if (mem_wstrb[0]) memory[word_addr][ 7: 0] <= mem_wdata[ 7: 0];
                    if (mem_wstrb[1]) memory[word_addr][15: 8] <= mem_wdata[15: 8];
                    if (mem_wstrb[2]) memory[word_addr][23:16] <= mem_wdata[23:16];
                    if (mem_wstrb[3]) memory[word_addr][31:24] <= mem_wdata[31:24];
                end else if (led_sel) begin
                    // Active-low LEDs: invert in hardware so software writes
                    // a 1 bit to light a LED.
                    leds <= ~mem_wdata[5:0];
                end
                // else: unmapped — silently swallow.
            end else begin
                // Read path
                if (ram_sel) begin
                    mem_rdata <= memory[word_addr];
                end else if (led_sel) begin
                    // Read-back the *logical* (un-inverted) LED state
                    mem_rdata <= {26'b0, ~leds};
                end else begin
                    mem_rdata <= 32'h0;
                end
            end
        end
    end
endmodule

`default_nettype wire
