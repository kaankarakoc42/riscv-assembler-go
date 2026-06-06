// uart_adapter.v
// =====================================================================
// nandland UART modüllerini (uart_rx.v, uart_tx.v) loader_fsm'in
// beklediği arayüze adapte eden ince wrapper.
//
// Loader FSM'imin port adlandırması:
//   rx_byte_valid, rx_byte    -- byte alındı pulse + data
//   tx_send, tx_data          -- byte gönder pulse + data
//   tx_busy                   -- gönderim devam ediyor
//
// nandland UART'ın port adlandırması:
//   o_RX_DV, o_RX_Byte
//   i_TX_DV, i_TX_Byte
//   o_TX_Active
//
// Bu wrapper sadece **port eşleme** yapar -- ek mantık yoktur.
// Lisans: nandland UART MIT lisanslıdır, UART_LICENSE dosyasına bakın.
// =====================================================================

`default_nettype none

module uart_adapter #(
    parameter integer CLKS_PER_BIT = 234     // 27 MHz / 115200 baud
) (
    input  wire       clk,

    // Fiziksel pinler (Tang Nano 9K: pin 17 RX, pin 18 TX)
    input  wire       uart_rx_pin,
    output wire       uart_tx_pin,

    // Loader FSM arayüzü
    output wire       rx_byte_valid,
    output wire [7:0] rx_byte,

    input  wire       tx_send,
    input  wire [7:0] tx_data,
    output wire       tx_busy
);

    // ─── RX: nandland uart_rx ──────────────────────────────────────────
    uart_rx #(.CLKS_PER_BIT(CLKS_PER_BIT)) u_rx (
        .i_Clock     (clk),
        .i_RX_Serial (uart_rx_pin),
        .o_RX_DV     (rx_byte_valid),     // DV pulse -> byte_valid pulse
        .o_RX_Byte   (rx_byte)
    );

    // ─── TX: nandland uart_tx ──────────────────────────────────────────
    // o_TX_Done sinyalini kullanmıyoruz; o_TX_Active'i tx_busy olarak
    // doğrudan bağlıyoruz. nandland TX'i IDLE'a döndüğünde Active=0
    // olur, böylece loader bir sonraki yanıtı gönderebilir.
    wire tx_done_unused;
    uart_tx #(.CLKS_PER_BIT(CLKS_PER_BIT)) u_tx (
        .i_Clock     (clk),
        .i_TX_DV     (tx_send),
        .i_TX_Byte   (tx_data),
        .o_TX_Active (tx_busy),
        .o_TX_Serial (uart_tx_pin),
        .o_TX_Done   (tx_done_unused)
    );

endmodule

`default_nettype wire
