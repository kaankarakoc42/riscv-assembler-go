// bram.v — Single-port synchronous BRAM for PicoRV32 instruction + data memory.
//
// Synthesizes to block RAM on every FPGA family we care about.
// Initialized at simulation/synth time from a $readmemh-compatible file.
//
// Address bus is byte-addressed externally; we discard the bottom two bits
// internally because PicoRV32 always issues 32-bit aligned word accesses
// when used in this configuration.
//
// Size: 2^WORD_BITS 32-bit words = 4 * 2^WORD_BITS bytes.
// Default: 11 → 2 KiW = 8 KiB. Tang Nano 9K has 26 BSRAM blocks each 2 KiB;
// 8 KiB fits easily.
`default_nettype none

module bram #(
    parameter integer WORD_BITS = 11,           // 2^11 words = 8 KiB
    parameter         INIT_FILE = "program.mem" // $readmemh source
) (
    input  wire                     clk,
    input  wire                     en,
    input  wire [3:0]               we,         // per-byte write strobes
    input  wire [WORD_BITS+1:0]     addr,       // byte address (low 2 bits unused)
    input  wire [31:0]              wdata,
    output reg  [31:0]              rdata
);
    localparam integer DEPTH = (1 << WORD_BITS);

    reg [31:0] mem [0:DEPTH-1];

    // Initialize both .text and .data from the same .mem file. The linker
    // emits a single concatenated mem image when data_at = rom (default).
    initial begin
        $readmemh(INIT_FILE, mem);
    end

    wire [WORD_BITS-1:0] word_addr = addr[WORD_BITS+1:2];

    always @(posedge clk) begin
        if (en) begin
            // Read first, then mask in writes (read-before-write semantics).
            rdata <= mem[word_addr];

            if (we[0]) mem[word_addr][ 7: 0] <= wdata[ 7: 0];
            if (we[1]) mem[word_addr][15: 8] <= wdata[15: 8];
            if (we[2]) mem[word_addr][23:16] <= wdata[23:16];
            if (we[3]) mem[word_addr][31:24] <= wdata[31:24];
        end
    end
endmodule

`default_nettype wire
