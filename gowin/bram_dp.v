// bram_dp.v
// Semi-Dual-Port 32-bit BRAM (GW1NR-9 SDP modu) + Boot ROM init
// =============================================================
//
// Port A: yazma portu (yazilim loader CPU uzerinden buraya yazar)
// Port B: salt-okuma portu (CPU fetch/load)
//
// GW1NR-9 BSRAM Semi-Dual-Port (SDP) modu:
//   - Port A: write-only
//   - Port B: read-only
//   - WRITE_MODE = NORMAL (DPB'nin desteklemedigi READ_BEFORE_WRITE yok)
//
// Boot ROM:
//   INIT_FILE verilirse mem dizisi $readmemh ile loader makine koduyla
//   doldurulur. CPU reset'te 0x0'dan (loader) calismaya baslar. Gowin
//   sentezi block_ram init'i icin bu yontemi destekler.
//
// NOT (onceki tasarimdan fark): Bu surumde Port A artik loader_fsm yerine
// CPU veriyolu tarafindan (dispatcher araciligiyla) surulur. CPU'nun
// kullanici programini RAM'e yazmasi ve stack islemleri Port A uzerinden,
// fetch/load Port B uzerinden gerceklesir. PicoRV32 ayni anda hem okuyup
// hem yazmadigi icin SDP cakismasi olmaz.

`default_nettype none

module bram_dp #(
    parameter integer AW        = 11,        // 2^11 = 2048 word = 8 KB
    parameter         INIT_FILE = ""         // "" ise init yok
) (
    input  wire             clk,

    // Port A: yazma portu
    input  wire             a_we,
    input  wire [AW-1:0]    a_addr,
    input  wire [31:0]      a_wdata,

    // Port B: salt-okuma portu
    input  wire             b_en,
    input  wire [AW-1:0]    b_addr,
    input  wire [3:0]       b_wstrb,   // BAGLI degil — uyumluluk icin var
    input  wire [31:0]      b_wdata,   // BAGLI degil — uyumluluk icin var
    output reg  [31:0]      b_rdata
);

    // Kullanilmayan portlari gor; sentezleyici uyarmasin
    wire _unused = &{1'b0, b_wstrb, b_wdata, b_en, 1'b0};

    (* syn_ramstyle = "block_ram" *) reg [31:0] mem [0:(1<<AW)-1];

    // Boot ROM init
    initial begin
        if (INIT_FILE != "")
            $readmemh(INIT_FILE, mem);
    end

    // Port A: senkron yazma
    always @(posedge clk) begin
        if (a_we)
            mem[a_addr] <= a_wdata;
    end

    // Port B: senkron okuma
    always @(posedge clk) begin
        b_rdata <= mem[b_addr];
    end

endmodule

`default_nettype wire
