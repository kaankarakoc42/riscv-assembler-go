// uart_mmio.v
// =====================================================================
// nandland UART'i (uart_adapter uzerinden) PicoRV32'nin bellek-eslemeli
// (MMIO) yazmaclarina baglar. Yazilim loader (loader.s) bu yazmaclari
// polling ile kullanir.
//
// Yazmac haritasi (taban 0x8000_0008, dispatcher tarafindan cozulur):
//   +0  RX_DATA  (oku) : bit[7:0] son gelen bayt
//   +4  RX_READY (oku) : bit0 = okunmamis bayt var; RX_DATA okunca temizlenir
//   +8  TX_DATA  (yaz) : bit[7:0] gonderilecek bayt
//   +12 TX_BUSY  (oku) : bit0 = verici mesgul
//
// TX_BUSY notu: nandland uart_tx, i_TX_DV'den BIR cevrim sonra o_TX_Active'i
// yukseltir. Bu bosluk icin "tx_started" sticky biti kullanilir; boylece
// yazma anindan iletim sonuna kadar busy kesintisiz 1 kalir ve loader
// pespese bayt gonderirken bayt kaybetmez.
// =====================================================================

`default_nettype none

module uart_mmio (
    input  wire        clk,
    input  wire        resetn,

    // ── uart_adapter tarafi ────────────────────────────────────────────
    input  wire        rx_byte_valid,   // 1 cevrimlik pulse
    input  wire [7:0]  rx_byte,
    output reg         tx_send,         // 1 cevrimlik pulse -> i_TX_DV
    output reg  [7:0]  tx_data,
    input  wire        tx_busy,         // o_TX_Active

    // ── CPU (dispatcher) tarafi ─────────────────────────────────────────
    input  wire        rx_read_stb,     // CPU RX_DATA'yi okudu (1 cevrim)
    input  wire        tx_write_stb,    // CPU TX_DATA'ya yazdi (1 cevrim)
    input  wire [7:0]  cpu_wdata,       // gonderilecek bayt
    output wire [7:0]  rx_data,         // tutulan bayt
    output reg         rx_ready,        // okunmamis bayt var mi
    output wire        tx_busy_out      // birlesik busy
);

    // ── RX tutma yazmaci ────────────────────────────────────────────────
    reg [7:0] rx_hold;
    assign rx_data = rx_hold;

    always @(posedge clk or negedge resetn) begin
        if (!resetn) begin
            rx_hold  <= 8'h00;
            rx_ready <= 1'b0;
        end else begin
            // Once okuma tuketimi, sonra yeni bayt yakalama:
            // ayni cevrimde ikisi de olursa yeni bayt korunur (kayip yok).
            if (rx_read_stb)
                rx_ready <= 1'b0;
            if (rx_byte_valid) begin
                rx_hold  <= rx_byte;
                rx_ready <= 1'b1;
            end
        end
    end

    // ── TX pulse uretimi + sticky busy ──────────────────────────────────
    reg tx_started;
    assign tx_busy_out = tx_busy | tx_started;

    always @(posedge clk or negedge resetn) begin
        if (!resetn) begin
            tx_send    <= 1'b0;
            tx_data    <= 8'h00;
            tx_started <= 1'b0;
        end else begin
            tx_send <= 1'b0;
            if (tx_write_stb) begin
                tx_send    <= 1'b1;
                tx_data    <= cpu_wdata;
                tx_started <= 1'b1;        // Active yukselene kadar kopru
            end else if (tx_busy) begin
                tx_started <= 1'b0;        // Active yukseldi, artik tx_busy kapsiyor
            end
        end
    end

endmodule

`default_nettype wire
