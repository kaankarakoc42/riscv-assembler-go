module top_module (
    input clk,
    input resetn,     // Düğmeye basıldığında sistemi sıfırlamak için
    output reg [5:0] leds =6'b111111 // Tang Nano 9K üzerindeki 6 LED
);

    // İşlemci ile bellek arasındaki sinyaller
    wire [31:0] mem_addr;
    wire [31:0] mem_wdata;
    reg  [31:0] mem_rdata;
    wire mem_valid;
    wire mem_instr;
    reg  mem_ready;
    wire [3:0] mem_wstrb;

    // PicoRV32 Çekirdeğini Çağırma (İsimler sizin projenizdeki PicoRV32 modülüne göre değişebilir)
    picorv32 cpu (
        .clk(clk),
        .resetn(resetn),
        .mem_valid(mem_valid),
        .mem_instr(mem_instr),
        .mem_ready(mem_ready),
        .mem_addr(mem_addr),
        .mem_wdata(mem_wdata),
        .mem_wstrb(mem_wstrb),
        .mem_rdata(mem_rdata)
    );

    // 1024 Kelimelik (4KB) Bellek Tanımlaması
    reg [31:0] memory [0:1023];

    // Ürettiğiniz Hex dosyasını belleğe yükleyin
    initial begin
        $readmemh("program.hex", memory);
    end

    // Bellek Okuma ve Yazma (Memory-Mapped I/O)
    always @(posedge clk) begin
        mem_ready <= 0;
        
        if (mem_valid && !mem_ready) begin
            mem_ready <= 1; // İşlemciye verinin hazır olduğunu/alındığını bildir

            // Yazma İşlemi (mem_wstrb 0'dan farklıysa)
            if (|mem_wstrb) begin
                // LED Adresine (0x40000000) yazma yapılıyorsa
                if (mem_addr == 32'h40000000) begin
                    // LED'ler genellikle Low-Active (0'da yanar) olduğu için tersliyoruz
                    leds <= ~mem_wdata[5:0]; 
                end 
                else begin
                    // Normal RAM'e yazma
                    if (mem_wstrb[0]) memory[mem_addr[11:2]][7:0]   <= mem_wdata[7:0];
                    if (mem_wstrb[1]) memory[mem_addr[11:2]][15:8]  <= mem_wdata[15:8];
                    if (mem_wstrb[2]) memory[mem_addr[11:2]][23:16] <= mem_wdata[23:16];
                    if (mem_wstrb[3]) memory[mem_addr[11:2]][31:24] <= mem_wdata[31:24];
                end
            end 
            // Okuma İşlemi
            else begin
                mem_rdata <= memory[mem_addr[11:2]]; // Adresi 4'e bölerek kelime hizalı oku
            end
        end
    end
endmodule