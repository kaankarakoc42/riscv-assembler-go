// tang_nano_top.v
// Tang Nano 9K - PicoRV32 + Yazilim Loader (Boot ROM) + SDP BRAM
// =============================================================
//
// 3. proje top dosyasi (yazilim-loader surumu). Donanim loader_fsm
// KALDIRILDI; loader artik BRAM'in basina gomulu bir RISC-V programi
// (loader.s -> loader.hex) olarak CPU uzerinde calisir.
//
// Akis:
//   Power-on:
//     1. sys_resetn cikinca PicoRV32 0x0'dan (loader) calismaya baslar
//   Yukleme (loader yazilimi):
//     2. loader UART MMIO'dan polling ile paketleri okur
//     3. CRC'yi yazilimda dogrular, DATA'yi RAM'e (Port A) yazar
//     4. START gelince entry adresine atlar (jr)
//   Calisma:
//     5. Kullanici programi calisir (fetch/load Port B, store Port A)
//     6. Yeniden yukleme icin S1 (sys_rst_n) basilir -> loader yeniden acilir
//
// Bellek haritasi:
//   0x0000_0000 - 0x0000_03FF   Loader kodu (reset vektoru)
//   0x0000_0400 - 0x0000_1FFF   Kullanici programi alani
//   0x8000_0000  LED (R/W)
//   0x8000_0004  BUTON (R, bit0 = S2)
//   0x8000_0008  UART RX_DATA (R)
//   0x8000_000C  UART RX_READY (R, bit0)
//   0x8000_0010  UART TX_DATA (W)
//   0x8000_0014  UART TX_BUSY (R, bit0)
//
// UART: 115200 baud, 8N1, Tang Nano 9K onboard BL702 (pin 17 RX, 18 TX)
// UART IP referansi: nandland (MIT) — UART_LICENSE.

`default_nettype none

module top (
    input  wire        sys_clk,       // 27 MHz
    input  wire        sys_rst_n,     // negatif aktif reset (S1)
    input  wire        button_s2,     // S2 (active-low pin -> 1=basili)
    output wire [5:0]  led,           // 6 LED (active-low)
    input  wire        uart_rx_pin,
    output wire        uart_tx_pin
);

    wire clk = sys_clk;

    // ─── Power-on reset zinciri ────────────────────────────────────────
    reg [5:0] reset_cnt = 0;
    wire sys_resetn = &reset_cnt;
    always @(posedge clk) begin
        if (!sys_rst_n)
            reset_cnt <= 0;
        else if (!sys_resetn)
            reset_cnt <= reset_cnt + 1;
    end

    // ─── PicoRV32 memory interface ─────────────────────────────────────
    wire        mem_valid;
    wire        mem_instr;
    reg         mem_ready;
    wire [31:0] mem_addr;
    wire [31:0] mem_wdata;
    wire [3:0]  mem_wstrb;
    reg  [31:0] mem_rdata;

    // ─── UART (nandland uart_rx/tx, adapter ile) ───────────────────────
    // 115200 baud @ 27 MHz -> 234 saat/bit
    wire       uart_rx_valid;
    wire [7:0] uart_rx_data;
    wire       uart_tx_send;
    wire [7:0] uart_tx_data;
    wire       uart_tx_busy;

    uart_adapter #(.CLKS_PER_BIT(234)) u_uart (
        .clk           (clk),
        .uart_rx_pin   (uart_rx_pin),
        .uart_tx_pin   (uart_tx_pin),
        .rx_byte_valid (uart_rx_valid),
        .rx_byte       (uart_rx_data),
        .tx_send       (uart_tx_send),
        .tx_data       (uart_tx_data),
        .tx_busy       (uart_tx_busy)
    );

    // ─── UART MMIO ──────────────────────────────────────────────────────
    wire [7:0] umm_rx_data;
    wire       umm_rx_ready;
    wire       umm_tx_busy;
    reg        rx_read_stb;
    reg        tx_write_stb;

    uart_mmio u_umm (
        .clk           (clk),
        .resetn        (sys_resetn),
        .rx_byte_valid (uart_rx_valid),
        .rx_byte       (uart_rx_data),
        .tx_send       (uart_tx_send),
        .tx_data       (uart_tx_data),
        .tx_busy       (uart_tx_busy),
        .rx_read_stb   (rx_read_stb),
        .tx_write_stb  (tx_write_stb),
        .cpu_wdata     (mem_wdata[7:0]),
        .rx_data       (umm_rx_data),
        .rx_ready      (umm_rx_ready),
        .tx_busy_out   (umm_tx_busy)
    );

    // ─── Adres cozme ────────────────────────────────────────────────────
    wire bram_sel = (mem_addr[31:28] == 4'h0);   // 0x0xxx_xxxx
    wire mmio_sel = (mem_addr[31:28] == 4'h8);   // 0x8xxx_xxxx
    wire [10:0] bram_word_idx = mem_addr[12:2];
    wire [3:0]  mmio_reg_sel  = mem_addr[5:2];

    // ─── BRAM portlari ──────────────────────────────────────────────────
    // Port A: CPU yazma (store / loader'in program yazimi)
    reg         a_we;
    reg  [10:0] a_addr;
    reg  [31:0] a_wdata;
    // Port B: CPU okuma (fetch / load)
    // b_addr KOMBINASYONEL: senkron BRAM, adresi sundugumuz cevrimin
    // sonunda b_rdata'yi uretir; boylece S_BRAM_WAIT'te dogru veriyi
    // yakalariz (bir cevrim erken yakalama hatasi giderildi).
    reg         b_en;
    wire [10:0] b_addr = bram_word_idx;
    wire [31:0] b_rdata;

    // ─── LED MMIO ───────────────────────────────────────────────────────
    reg [7:0] led_reg = 8'h00;
    assign led = ~led_reg[5:0];

    // ─── Buton senkronizasyonu (2 FF) ──────────────────────────────────
    reg btn_sync0, btn_sync1;
    always @(posedge clk) begin
        btn_sync0 <= ~button_s2;
        btn_sync1 <= btn_sync0;
    end
    wire btn_stable = btn_sync1;

    // ─── Dispatcher ─────────────────────────────────────────────────────
    // S_IDLE:  BRAM yazma -> Port A, ayni cevrim ACK
    //          BRAM okuma -> Port B, S_BRAM_WAIT
    //          MMIO       -> ayni cevrim ACK
    // S_BRAM_WAIT: 1 cevrim sonra b_rdata gecerli, ACK
    localparam S_IDLE = 1'b0, S_BRAM_WAIT = 1'b1;
    reg disp_state;

    always @(posedge clk or negedge sys_resetn) begin
        if (!sys_resetn) begin
            disp_state   <= S_IDLE;
            mem_ready    <= 1'b0;
            mem_rdata    <= 32'h0;
            led_reg      <= 8'h00;
            a_we         <= 1'b0;
            a_addr       <= 11'h0;
            a_wdata      <= 32'h0;
            b_en         <= 1'b0;
            rx_read_stb  <= 1'b0;
            tx_write_stb <= 1'b0;
        end else begin
            // varsayilan: tek-cevrimlik pulse'lar dusuk
            mem_ready    <= 1'b0;
            a_we         <= 1'b0;
            b_en         <= 1'b0;
            rx_read_stb  <= 1'b0;
            tx_write_stb <= 1'b0;

            case (disp_state)
            S_IDLE: begin
                if (mem_valid && !mem_ready) begin
                    if (bram_sel) begin
                        if (mem_wstrb != 4'b0000) begin
                            // BRAM yazma (tam word) -> Port A
                            a_we      <= 1'b1;
                            a_addr    <= bram_word_idx;
                            a_wdata   <= mem_wdata;
                            mem_ready <= 1'b1;
                        end else begin
                            // BRAM okuma -> Port B
                            b_en       <= 1'b1;
                            disp_state <= S_BRAM_WAIT;
                        end
                    end else if (mmio_sel) begin
                        case (mmio_reg_sel)
                            4'h0: begin   // LED (R/W)
                                if (mem_wstrb != 0)
                                    led_reg <= mem_wdata[7:0];
                                mem_rdata <= {24'b0, led_reg};
                            end
                            4'h1: begin   // BUTON (R)
                                mem_rdata <= {31'b0, btn_stable};
                            end
                            4'h2: begin   // UART RX_DATA (R)
                                mem_rdata <= {24'b0, umm_rx_data};
                                if (mem_wstrb == 0)
                                    rx_read_stb <= 1'b1;
                            end
                            4'h3: begin   // UART RX_READY (R)
                                mem_rdata <= {31'b0, umm_rx_ready};
                            end
                            4'h4: begin   // UART TX_DATA (W)
                                if (mem_wstrb != 0)
                                    tx_write_stb <= 1'b1;
                                mem_rdata <= 32'h0;
                            end
                            4'h5: begin   // UART TX_BUSY (R)
                                mem_rdata <= {31'b0, umm_tx_busy};
                            end
                            default: mem_rdata <= 32'h0;
                        endcase
                        mem_ready <= 1'b1;
                    end else begin
                        mem_rdata <= 32'h0;
                        mem_ready <= 1'b1;
                    end
                end
            end

            S_BRAM_WAIT: begin
                mem_rdata  <= b_rdata;
                mem_ready  <= 1'b1;
                disp_state <= S_IDLE;
            end

            default: disp_state <= S_IDLE;
            endcase
        end
    end

    // ─── Dual-port BRAM (boot ROM = loader.hex) ─────────────────────────
    bram_dp #(
        .AW        (11),
        .INIT_FILE ("loader.hex")
    ) u_bram (
        .clk     (clk),
        .a_we    (a_we),
        .a_addr  (a_addr),
        .a_wdata (a_wdata),
        .b_en    (b_en),
        .b_addr  (b_addr),
        .b_wstrb (4'b0),
        .b_wdata (32'b0),
        .b_rdata (b_rdata)
    );

    // ─── PicoRV32 ──────────────────────────────────────────────────────
    picorv32 #(
        .ENABLE_COUNTERS      (0),
        .ENABLE_REGS_DUALPORT (1),
        .CATCH_MISALIGN       (0),
        .CATCH_ILLINSN        (0),
        .COMPRESSED_ISA       (0),
        .ENABLE_MUL           (0),
        .ENABLE_DIV           (0),
        .PROGADDR_RESET       (32'h0000_0000)
    ) cpu (
        .clk         (clk),
        .resetn      (sys_resetn),      // <-- artik dogrudan sistem reseti
        .mem_valid   (mem_valid),
        .mem_instr   (mem_instr),
        .mem_ready   (mem_ready),
        .mem_addr    (mem_addr),
        .mem_wdata   (mem_wdata),
        .mem_wstrb   (mem_wstrb),
        .mem_rdata   (mem_rdata),
        .mem_la_read (),
        .mem_la_write(),
        .mem_la_addr (),
        .mem_la_wdata(),
        .mem_la_wstrb(),
        .pcpi_valid  (),
        .pcpi_insn   (),
        .pcpi_rs1    (),
        .pcpi_rs2    (),
        .pcpi_wr     (1'b0),
        .pcpi_rd     (32'h0),
        .pcpi_wait   (1'b0),
        .pcpi_ready  (1'b0),
        .irq         (32'h0),
        .eoi         (),
        .trace_valid (),
        .trace_data  ()
    );

endmodule

`default_nettype wire
