// soc_top.v — Minimal PicoRV32 SoC for Tang Nano 9K.
//
// Provides:
//
//   * 8 KiB BRAM, byte-write enabled, initialized from "program.mem"
//   * Memory-mapped LEDs at  0x80000000 (low 6 bits drive the 6 LEDs).
//   * Memory-mapped UART TX  at  0x80000010 (write any byte; bit-banged TX).
//   * Memory-mapped milli-tick counter at 0x80000020 (read-only)
//
// PicoRV32 is included as a separately-sourced module ("picorv32"). Get it
// from https://github.com/YosysHQ/picorv32 and add picorv32.v to your
// synthesis project alongside the files in this directory.
//
// PARAMETERS YOU MAY WANT TO CHANGE
//
//   CLK_HZ      crystal frequency (Tang Nano 9K = 27 MHz)
//   UART_BAUD   bit rate for the bit-banged TX
//   ROM_KIB     instruction memory size in KiB (must match linker script)
`default_nettype none

module soc_top #(
    parameter integer CLK_HZ    = 27_000_000,
    parameter integer UART_BAUD = 115_200,
    parameter integer ROM_KIB   = 8
) (
    input  wire        clk,
    input  wire        rst_n,        // active-low (Tang Nano 9K's S1 button)
    output wire [5:0]  leds,         // active-low on Tang Nano 9K
    output wire        uart_tx
);
    // ───── Reset synchronizer ──────────────────────────────────────────
    reg [3:0] rst_sync = 4'b0;
    wire      resetn   = rst_sync[3];
    always @(posedge clk) rst_sync <= {rst_sync[2:0], rst_n};

    // ───── PicoRV32 native memory bus ──────────────────────────────────
    wire        mem_valid;
    wire        mem_instr;
    reg         mem_ready;
    wire [31:0] mem_addr;
    wire [31:0] mem_wdata;
    wire [3:0]  mem_wstrb;
    reg  [31:0] mem_rdata;

    picorv32 #(
        .ENABLE_COUNTERS  (0),
        .ENABLE_REGS_DUALPORT(1),
        .COMPRESSED_ISA   (0),
        .BARREL_SHIFTER   (0),
        .ENABLE_MUL       (0),
        .ENABLE_DIV       (0),
        .ENABLE_FAST_MUL  (0),
        .ENABLE_IRQ       (0)
    ) cpu (
        .clk      (clk),
        .resetn   (resetn),
        .mem_valid(mem_valid),
        .mem_instr(mem_instr),
        .mem_ready(mem_ready),
        .mem_addr (mem_addr),
        .mem_wdata(mem_wdata),
        .mem_wstrb(mem_wstrb),
        .mem_rdata(mem_rdata),
        // tied off / unused
        .pcpi_valid(),
        .pcpi_insn (),
        .pcpi_rs1  (),
        .pcpi_rs2  (),
        .pcpi_wr   (1'b0),
        .pcpi_rd   (32'b0),
        .pcpi_wait (1'b0),
        .pcpi_ready(1'b0),
        .irq       (32'b0),
        .eoi       ()
    );

    // ───── Address decoding ─────────────────────────────────────────────
    //
    //  0x00000000 .. ROM_KIB*1024-1   : BRAM (text + data)
    //  0x80000000                     : LED register (write-only)
    //  0x80000010                     : UART TX register (write-only)
    //  0x80000020                     : ms-tick counter (read-only)
    localparam integer ROM_WORD_BITS = $clog2((ROM_KIB*1024)/4);

    wire sel_rom  = (mem_addr[31] == 1'b0);
    wire sel_io   = (mem_addr[31:24] == 8'h80);
    wire sel_led  = sel_io && (mem_addr[7:0] == 8'h00);
    wire sel_uart = sel_io && (mem_addr[7:0] == 8'h10);
    wire sel_tick = sel_io && (mem_addr[7:0] == 8'h20);

    // ───── BRAM ─────────────────────────────────────────────────────────
    wire [31:0] rom_rdata;
    bram #(
        .WORD_BITS(ROM_WORD_BITS),
        .INIT_FILE("program.mem")
    ) rom (
        .clk   (clk),
        .en    (mem_valid && sel_rom),
        .we    (mem_valid && sel_rom ? mem_wstrb : 4'b0),
        .addr  (mem_addr[ROM_WORD_BITS+1:0]),
        .wdata (mem_wdata),
        .rdata (rom_rdata)
    );

    // ───── LEDs (write-only register) ──────────────────────────────────
    // Tang Nano 9K LEDs are active-low; we invert here so software can
    // pretend they're active-high.
    reg [5:0] led_reg = 6'h00;
    assign leds = ~led_reg;

    always @(posedge clk) begin
        if (mem_valid && sel_led && |mem_wstrb) begin
            led_reg <= mem_wdata[5:0];
        end
    end

    // ───── Bit-banged UART TX ──────────────────────────────────────────
    localparam integer UART_DIV = CLK_HZ / UART_BAUD;
    reg [3:0]                    uart_bit;
    reg [$clog2(UART_DIV)-1:0]   uart_cnt;
    reg [9:0]                    uart_shift;
    reg                          uart_busy = 1'b0;
    reg                          uart_out  = 1'b1;
    assign uart_tx = uart_out;

    always @(posedge clk) begin
        if (uart_busy) begin
            if (uart_cnt == UART_DIV - 1) begin
                uart_cnt   <= 0;
                uart_out   <= uart_shift[0];
                uart_shift <= {1'b1, uart_shift[9:1]};
                if (uart_bit == 9) begin
                    uart_busy <= 1'b0;
                    uart_out  <= 1'b1;
                end else begin
                    uart_bit <= uart_bit + 1;
                end
            end else begin
                uart_cnt <= uart_cnt + 1;
            end
        end else if (mem_valid && sel_uart && |mem_wstrb) begin
            // start bit (0), 8 data bits, stop bit (1)
            uart_shift <= {1'b1, mem_wdata[7:0], 1'b0};
            uart_bit   <= 0;
            uart_cnt   <= 0;
            uart_out   <= 1'b0;
            uart_busy  <= 1'b1;
        end
    end

    // ───── Millisecond tick counter ────────────────────────────────────
    localparam integer MS_DIV = CLK_HZ / 1000;
    reg [$clog2(MS_DIV)-1:0] ms_cnt;
    reg [31:0]               ticks;
    always @(posedge clk) begin
        if (ms_cnt == MS_DIV - 1) begin
            ms_cnt <= 0;
            ticks  <= ticks + 1;
        end else begin
            ms_cnt <= ms_cnt + 1;
        end
    end

    // ───── Bus reply / arbitration ─────────────────────────────────────
    // BRAM has 1-cycle latency; IO is combinational. We accept one
    // transaction per cycle and ack on the next.
    reg sel_rom_d;
    always @(posedge clk) sel_rom_d <= mem_valid && sel_rom;

    always @(*) begin
        mem_ready = 1'b0;
        mem_rdata = 32'h0;
        if (mem_valid) begin
            if (sel_rom) begin
                mem_ready = sel_rom_d; // ack one cycle later
                mem_rdata = rom_rdata;
            end else if (sel_led) begin
                mem_ready = 1'b1;
                mem_rdata = {26'b0, led_reg};
            end else if (sel_uart) begin
                mem_ready = !uart_busy; // back-pressure when busy
                mem_rdata = {31'b0, uart_busy};
            end else if (sel_tick) begin
                mem_ready = 1'b1;
                mem_rdata = ticks;
            end else begin
                // Unmapped — ack and return 0 so we don't deadlock.
                mem_ready = 1'b1;
                mem_rdata = 32'h0;
            end
        end
    end

endmodule

`default_nettype wire
