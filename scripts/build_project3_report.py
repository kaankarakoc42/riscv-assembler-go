from __future__ import annotations

from pathlib import Path

from docx import Document
from docx.enum.section import WD_SECTION
from docx.enum.table import WD_ALIGN_VERTICAL, WD_TABLE_ALIGNMENT
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.shared import Inches, Pt, RGBColor


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "build" / "reports"
DOCX = OUT / "BIL302_PROJE3_RV32I_LOADER_RAPORU.docx"


def set_cell_shading(cell, fill: str) -> None:
    tc_pr = cell._tc.get_or_add_tcPr()
    shd = tc_pr.find(qn("w:shd"))
    if shd is None:
        shd = OxmlElement("w:shd")
        tc_pr.append(shd)
    shd.set(qn("w:fill"), fill)


def set_cell_text(cell, text: str, bold: bool = False) -> None:
    cell.text = ""
    p = cell.paragraphs[0]
    p.paragraph_format.space_after = Pt(0)
    r = p.add_run(text)
    r.font.name = "Courier New"
    r._element.rPr.rFonts.set(qn("w:eastAsia"), "Courier New")
    r.font.size = Pt(10)
    r.bold = bold


def add_table(doc: Document, headers: list[str], rows: list[list[str]], widths: list[float] | None = None):
    table = doc.add_table(rows=1, cols=len(headers))
    table.alignment = WD_TABLE_ALIGNMENT.CENTER
    table.style = "Table Grid"
    table.autofit = False
    hdr = table.rows[0].cells
    for i, h in enumerate(headers):
        set_cell_text(hdr[i], h, True)
        set_cell_shading(hdr[i], "E8EEF5")
    for row in rows:
        cells = table.add_row().cells
        for i, text in enumerate(row):
            set_cell_text(cells[i], text)
            cells[i].vertical_alignment = WD_ALIGN_VERTICAL.CENTER
    if widths:
        for row in table.rows:
            for i, w in enumerate(widths):
                row.cells[i].width = Inches(w)
    doc.add_paragraph()
    return table


def add_code(doc: Document, text: str) -> None:
    p = doc.add_paragraph()
    p.style = doc.styles["Code"]
    for line in text.rstrip().splitlines():
        r = p.add_run(line + "\n")
        r.font.name = "Courier New"
        r._element.rPr.rFonts.set(qn("w:eastAsia"), "Courier New")
        r.font.size = Pt(9)


def add_bullet(doc: Document, text: str) -> None:
    p = doc.add_paragraph(style="List Bullet")
    p.add_run(text)


def add_number(doc: Document, text: str) -> None:
    p = doc.add_paragraph(style="List Number")
    p.add_run(text)


def add_image_if_exists(doc: Document, rel: str, caption: str, width: float = 5.9) -> None:
    path = ROOT / rel
    if not path.exists():
        return
    p = doc.add_paragraph()
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    p.add_run().add_picture(str(path), width=Inches(width))
    cap = doc.add_paragraph(caption)
    cap.alignment = WD_ALIGN_PARAGRAPH.CENTER
    cap.runs[0].italic = True


def p(doc: Document, text: str = "", style: str | None = None, bold_lead: str | None = None):
    para = doc.add_paragraph(style=style)
    if bold_lead and text.startswith(bold_lead):
        run = para.add_run(bold_lead)
        run.bold = True
        para.add_run(text[len(bold_lead):])
    else:
        para.add_run(text)
    return para


def heading(doc: Document, text: str, level: int = 1):
    return doc.add_heading(text, level=level)


def configure(doc: Document) -> None:
    sec = doc.sections[0]
    sec.page_width = Inches(8.5)
    sec.page_height = Inches(11)
    sec.top_margin = Inches(1)
    sec.bottom_margin = Inches(1)
    sec.left_margin = Inches(1)
    sec.right_margin = Inches(1)
    sec.header_distance = Inches(0.49)
    sec.footer_distance = Inches(0.49)

    styles = doc.styles
    normal = styles["Normal"]
    normal.font.name = "Courier New"
    normal._element.rPr.rFonts.set(qn("w:eastAsia"), "Courier New")
    normal.font.size = Pt(10)
    normal.paragraph_format.space_after = Pt(6)
    normal.paragraph_format.line_spacing = 1.15

    for name, size, color in [
        ("Title", 14, RGBColor(0x0B, 0x25, 0x45)),
        ("Heading 1", 12, RGBColor(0x1F, 0x4D, 0x78)),
        ("Heading 2", 11, RGBColor(0x2E, 0x74, 0xB5)),
        ("Heading 3", 10, RGBColor(0x1F, 0x4D, 0x78)),
    ]:
        s = styles[name]
        s.font.name = "Courier New"
        s._element.rPr.rFonts.set(qn("w:eastAsia"), "Courier New")
        s.font.size = Pt(size)
        s.font.color.rgb = color
        s.font.bold = True
        s.paragraph_format.space_before = Pt(10 if name != "Title" else 0)
        s.paragraph_format.space_after = Pt(5)

    code = styles.add_style("Code", 1)
    code.font.name = "Courier New"
    code._element.rPr.rFonts.set(qn("w:eastAsia"), "Courier New")
    code.font.size = Pt(9)
    code.paragraph_format.left_indent = Inches(0.2)
    code.paragraph_format.space_before = Pt(3)
    code.paragraph_format.space_after = Pt(6)

    for sname in ["List Bullet", "List Number"]:
        s = styles[sname]
        s.font.name = "Courier New"
        s._element.rPr.rFonts.set(qn("w:eastAsia"), "Courier New")
        s.font.size = Pt(10)
        s.paragraph_format.space_after = Pt(3)

    hdr = sec.header.paragraphs[0]
    hdr.text = "BIL302 Proje 3 - RV32I FPGA Loader"
    hdr.alignment = WD_ALIGN_PARAGRAPH.RIGHT
    hdr.runs[0].font.name = "Courier New"
    hdr.runs[0].font.size = Pt(8)

    ftr = sec.footer.paragraphs[0]
    ftr.text = "Rapor: PicoRV32 RV32I Loader Tasarımı"
    ftr.alignment = WD_ALIGN_PARAGRAPH.CENTER
    ftr.runs[0].font.name = "Courier New"
    ftr.runs[0].font.size = Pt(8)


def build() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    doc = Document()
    configure(doc)

    title = doc.add_paragraph(style="Title")
    title.alignment = WD_ALIGN_PARAGRAPH.CENTER
    title.add_run("PicoRV İşlemci Alt Kümesi (RV32I) İçin FPGA Tabanlı Loader Tasarımı").bold = True
    sub = doc.add_paragraph()
    sub.alignment = WD_ALIGN_PARAGRAPH.CENTER
    sub.add_run("BIL302 - 3. Proje/Tasarım Raporu\nTeslim tarihi: 07.06.2026\nGrup üyeleri: A.XX, B.YY, C.ZZ").bold = True
    p(doc, "Bu rapor, öğrencilerin geliştirdiği assembler ve linker araç zincirini FPGA üzerinde çalışan yazılım tabanlı bir loader ile birleştiren sistem tasarımını açıklar. Çalışma; loader protokolü, UART haberleşmesi, hata denetimi, host uygulaması, PicoRV32 bellek haritası, test senaryoları ve sürdürülebilirlik etkilerini Program Çıktıları ile ilişkilendirerek sunar (PÇ1, PÇ6, PÇ7, PÇ8, PÇ12, PÇ13).")

    heading(doc, "Özet", 1)
    p(doc, "Projede RV32I assembly kaynak kodundan başlayıp FPGA üzerindeki PicoRV32 işlemcisine seri port üzerinden program yüklenmesine kadar uzanan uçtan uca bir araç zinciri kurulmuştur. Önceki projelerde geliştirilen `rvasm` assembler ve `rvld` linker çıktıları, bu projede UART üzerinden çalışan özgün bir software loader ile donanıma taşınmıştır. Loader assembly olarak yazılmış, mevcut assembler tarafından derlenmiş ve `gowin/loader.hex` adıyla BRAM başlangıç dosyasına dönüştürülmüştür.")
    p(doc, "FPGA tarafında loader, reset vektöründe çalışan küçük bir RV32I programıdır. UART MMIO yazmaçlarını polling ile okuyarak host tarafından gönderilen paketleri alır, 8 bit checksum ile doğrular, makine kodu word'lerini BRAM'in kullanıcı programı alanına yazar ve START paketi geldiğinde program giriş adresine `jr` ile dallanır. Host tarafında Python ile yazılan `scripts/host_loader.py`, `.bin` veya `$readmemh` formatındaki `.mem/.hex` dosyalarını paketlere bölerek seri porttan gönderir.")

    heading(doc, "1. Giriş ve Literatür Araştırması", 1)
    heading(doc, "1.1. Gömülü Sistemlerde Program Yükleme Mimarileri", 2)
    p(doc, "Gömülü sistemlerde program yükleme işlemi genellikle boot ROM, UART/SPI bootloader, JTAG programlama veya harici flash üzerinden yapılır. Bootloader yaklaşımında işlemci reset sonrasında küçük ve güvenilir bir kod parçasını çalıştırır; bu kod harici arayüzden gelen uygulama imajını belleğe yerleştirir ve daha sonra uygulamaya kontrol verir. RISC-V ekosisteminde bu yapı, açık ISA ve açık kaynak işlemci çekirdekleri sayesinde eğitim ve araştırma amaçlı sistemlerde kolayca gözlenebilir hale gelir [1], [3] (PÇ6).")
    p(doc, "JTAG çoğunlukla hata ayıklama ve üretim programlama için güçlüdür; ancak kullanıcı uygulamasını sınıfta ve hızlı demo ortamında değiştirmek için ek araç bağımlılığı yaratır. SPI flash kalıcı depolama sağlar; fakat flash programlama akışı FPGA geliştirme kartında daha fazla donanım ve zaman gerektirebilir. UART loader ise düşük pin sayısı, standart PC desteği ve kolay paketleme mantığı nedeniyle eğitim amaçlı PicoRV32 sisteminde uygun bir seçimdir [2], [4] (PÇ6).")
    heading(doc, "1.2. Seri Haberleşme ve Veri Doğrulama Protokolleri", 2)
    p(doc, "UART, saat hattı taşımayan asenkron bir seri haberleşme protokolüdür. Bu nedenle bit zamanlaması ve alıcı/verici baud oranı uyumu önemlidir. Projede Tang Nano 9K üzerinde 27 MHz sistem saatine göre 115200 baud, 8N1 biçimi seçilmiştir. Fiziksel aktarım sırasında byte kaybı veya bozulma riskine karşı paket sonunda hata denetimi gerekir (PÇ7).")
    p(doc, "Checksum, toplam tabanlı düşük maliyetli bir denetim sağlar; CRC ise burst hata yakalama kabiliyeti daha güçlü olan polinom tabanlı bir yöntemdir [5]. Bu projede loader'ın RV32I assembly boyutunu küçük tutmak ve 0x00000000-0x000003FF boot alanına sığdırmak için 8 bit checksum kullanılmıştır. Host uygulaması her pakette `cmd + addr bytes + words + data bytes` toplamının düşük 8 bitini gönderir; loader aynı toplamı hesaplayıp gelen checksum ile karşılaştırır. CRC daha güçlü bir alternatif olarak ileriki sürüm için ayrılmıştır (PÇ6, PÇ7).")
    add_table(doc, ["Yöntem", "Avantaj", "Dezavantaj", "Projede tercih durumu"], [
        ["Checksum", "Çok küçük kod, hızlı hesap", "Bazı hata örüntülerini kaçırabilir", "Boot alanına sığdığı için seçildi"],
        ["CRC-8/CRC-16", "Burst hataları daha iyi yakalar", "Daha fazla kod ve hesap maliyeti", "Gelecek sürüm alternatifi"],
        ["ACK/NACK + yeniden gönderim", "Veri kaybında toparlanır", "Host-loader protokolünü büyütür", "Kısmi ACK: K/E/S cevaplarıyla sağlandı"],
    ], [1.1, 1.7, 1.7, 2.0])

    heading(doc, "2. Sistem Mimarisi ve Donanım-Yazılım Ortak Tasarımı", 1)
    heading(doc, "2.1. Toolchain Arayüz Standartları", 2)
    p(doc, "Sistem üç ana yazılım/donanım sınırından oluşur: assembler-linker sınırı, linker-host sınırı ve host-loader UART sınırı. Assembler `.s` dosyasını RVOB1 relocatable object dosyasına çevirir. Linker RVOB1 dosyalarını hedef bellek haritasına göre birleştirerek `.bin`, Intel HEX `.hex` ve `$readmemh` uyumlu `.mem` üretir. FPGA BRAM'i `$readmemh` formatını beklediği için loader build akışında linker'ın `.mem` çıktısı `gowin/loader.hex` adıyla kopyalanmıştır; dosya adı FPGA tarafındaki `INIT_FILE(\"loader.hex\")` parametresiyle uyumludur (PÇ1, PÇ6).")
    add_table(doc, ["Aşama", "Girdi", "Çıktı", "Sorumluluk"], [
        ["rvasm", "gowin/loader.s", "build/loader/loader.ro", "RV32I instruction encoding ve relocation kaydı"],
        ["rvld", "loader.ro + loader_link.toml", "loader.bin/hex/mem", "Boot adresine yerleşim ve image üretimi"],
        ["Makefile", "build/loader/loader.mem", "gowin/loader.hex", "BRAM init dosyasını doğru ada taşıma"],
        ["host_loader.py", ".bin veya .mem/.hex", "UART paketleri", "Adresli yazma, checksum, START komutu"],
    ], [1.3, 1.6, 1.5, 2.1])
    p(doc, "Gerçek derleme sonucunda loader `.text` boyutu 444 byte olmuştur. Bu değer 0x00000000-0x000003FF aralığındaki 1 KiB boot alanına sığmaktadır; kullanıcı programı varsayılan olarak 0x00000400 adresinden itibaren yüklenir (PÇ1).")
    add_code(doc, """make loader
bin/rvasm.exe -o build/loader/loader.ro gowin/loader.s
bin/rvld.exe -script gowin/loader_link.toml -o build/loader/loader build/loader/loader.ro
copy /Y build\\loader\\loader.mem gowin\\loader.hex

Memory layout:
  .text @ 0x00000000 (444 bytes)
  .data @ 0x000001BC (0 bytes)""")

    heading(doc, "2.2. FPGA Loader ve PicoRV32 Bellek Haritası", 2)
    p(doc, "FPGA top modülünde PicoRV32 reset vektörü 0x00000000 olarak bırakılmıştır. BRAM ilk açılışta `loader.hex` ile başlar. Loader programı UART RX/TX MMIO yazmaçlarını kullanarak host paketlerini okur. Paket doğrulanınca gelen 32 bit word, PicoRV32'nin normal store işlemiyle BRAM Port A üzerinden hedef adrese yazılır. Bu yaklaşımda ayrı bir donanım loader FSM yerine, FSM davranışı RV32I yazılımı içinde gerçekleşir. Böylece loader da öğrencilerin geliştirdiği assembler tarafından derlenen gerçek bir assembly programı olur (PÇ1, PÇ12).")
    add_table(doc, ["Adres", "İşlev", "Erişim", "Loader kullanımı"], [
        ["0x00000000-0x000003FF", "Boot loader alanı", "Fetch/read", "Reset sonrası ilk çalışan kod"],
        ["0x00000400-0x00001FFF", "Kullanıcı programı alanı", "R/W", "Host'tan gelen program buraya yazılır"],
        ["0x80000008", "UART RX_DATA", "Read", "Gelen byte okunur"],
        ["0x8000000C", "UART RX_READY", "Read", "Byte hazır mı kontrol edilir"],
        ["0x80000010", "UART TX_DATA", "Write", "R/K/E/S cevap byte'ları gönderilir"],
        ["0x80000014", "UART TX_BUSY", "Read", "Gönderim tamamlanana kadar beklenir"],
    ], [1.6, 1.7, 1.0, 2.1])
    add_image_if_exists(doc, "docs/images/mermaid-jpeg/diagram-02-line-50.jpeg", "Şekil 1. Assembler-linker-FPGA araç zinciri.", 5.7)
    p(doc, "Loader protokolü küçük bir FSM gibi çalışır: READY gönder, magic byte bekle, komutu ve adresi oku, veri word'lerini al, checksum doğrula, ACK/ERR dön ve START komutunda uygulama giriş adresine dallan. Donanım tarafında işlemci reset altında tutulmak yerine loader kodunu çalıştırdığı için PC yönlendirme işlemi `jr entry` ile yazılım seviyesinde yapılır. Bu, PDF yönergesindeki FSM fikrini yazılım tabanlı loader olarak uyarlayan bilinçli bir co-design kararıdır (PÇ6, PÇ12).")
    add_code(doc, """frame = 0x55, cmd, addr[0], addr[1], addr[2], addr[3], words, data..., sum
sum   = low8(cmd + addr bytes + words + data bytes)
cmd=1 WRITE: words adet little-endian 32-bit word hedef adrese yazılır
cmd=2 START: checksum doğruysa entry adresine jr ile dallanılır
replies: 'R' ready, 'K' write ok, 'E' error, 'S' starting""")

    heading(doc, "3. Deneysel Çalışmalar, Test ve Analiz", 1)
    heading(doc, "3.1. Deney Tasarımı ve Test Senaryoları", 2)
    p(doc, "Testler üç seviyede tasarlanmıştır: araç zinciri testleri, loader derleme/doğrulama testleri ve FPGA üzerinde gözlenebilir uygulama testleri. Amaç yalnızca kodun derlenmesi değil; farklı karmaşıklıktaki assembly programlarının seri porttan yüklenebilir ve donanım I/O ile gözlenebilir olmasıdır (PÇ7).")
    add_table(doc, ["Senaryo", "Karmaşıklık", "Donanım gözlemi", "Kapsanan özellik"], [
        ["Blink/Knight Rider", "Döngü + delay çağrısı", "LED deseninin kayması", "Extern sembol, call, branch, MMIO store"],
        ["Counter", "Alt program + sayaç okuma", "LED'lerde düşük 6 bit sayaç", "read_ms/set_leds fonksiyonları, I/O soyutlama"],
        ["UART Hello", "Data section + string + UART", "Seri terminalde mesaj", "la pseudo, .data, uart_puts, delay_ms"],
        ["Loader checksum hatası", "Negatif test", "Host 'E' cevabı alır", "Paket doğrulama ve hata yönetimi"],
    ], [1.4, 1.5, 1.5, 2.1])
    p(doc, "Örnek test programı 1: LED blink. Bu program harici `delay` fonksiyonunu çağırır ve LED register'a farklı bit desenleri yazar.")
    add_code(doc, """.global main
.extern delay
main:
    li   t1, 0x40000000
    li   t2, 0x01
loop:
    sw   t2, 0(t1)
    li   a0, 0x40000
    call delay
    slli t2, t2, 1
    li   t3, 0x40
    bne  t2, t3, no_wrap
    li   t2, 0x01
no_wrap:
    j    loop""")
    p(doc, "Örnek test programı 2: counter. Bu program farklı dosyalardaki `read_ms` ve `set_leds` fonksiyonlarını kullanır; bu nedenle linker'ın extern sembol çözümlemesini sınar.")
    add_code(doc, """loop:
    call read_ms
    andi a0, a0, 0x3F
    call set_leds
    j    loop""")
    p(doc, "Örnek test programı 3: UART hello. Bu program `.data` içindeki string adresini `la` pseudo-instruction ile alır ve UART üzerinden düzenli mesaj gönderir.")
    add_code(doc, """loop:
    la   a0, msg
    call uart_puts
    li   a0, 1000
    call delay_ms
    j    loop

.data
msg:
    .asciz \"Hello, RV32I!\\r\\n\"""")

    heading(doc, "3.2. Veri Toplama ve Donanım Metrikleri", 2)
    p(doc, "Ölçülen yazılım çıktıları aşağıdaki gibidir. `go test ./...` tüm paketlerde başarılı tamamlanmıştır. Loader build'i sonucunda BRAM'e gömülen loader 444 byte, yani 111 adet 32 bit word olmuştur. Host script 16 word varsayılan paket boyutu kullanır; bu da her WRITE paketinde 64 byte program verisi taşınması anlamına gelir (PÇ7).")
    add_table(doc, ["Metrik", "Değer", "Kaynak"], [
        ["Loader kod boyutu", "444 byte / 111 word", "build/loader/loader.map"],
        ["Boot alanı bütçesi", "1024 byte", "gowin/loader_link.toml"],
        ["Varsayılan kullanıcı başlangıcı", "0x00000400", "host_loader.py --base default"],
        ["UART baud", "115200 8N1", "tang_nano_top.v ve host_loader.py"],
        ["Varsayılan paket veri alanı", "16 word / 64 byte", "host_loader.py --chunk-words"],
        ["Birim/entegrasyon testleri", "go test ./... başarılı", "Komut doğrulaması"],
    ], [2.0, 1.5, 2.6])
    p(doc, "Yükleme süresi için teorik alt sınır UART hat hızıyla hesaplanabilir. 115200 baud ve 8N1 biçiminde her byte yaklaşık 10 bit taşır. 64 byte veri içeren WRITE paketinde 1 magic + 1 cmd + 4 adres + 1 word sayısı + 64 veri + 1 checksum = 72 byte gönderilir. Bir paket yaklaşık 720 bit, yani 6.25 ms seri hat süresi gerektirir. ACK bekleme ve Python/USB gecikmesi eklendiğinde gerçek süre biraz daha büyür (PÇ7).")
    add_table(doc, ["Program boyutu", "Paket sayısı", "Yaklaşık hat süresi", "Açıklama"], [
        ["512 byte", "8", "yaklaşık 50 ms + ACK gecikmeleri", "Küçük LED demo sınıfı"],
        ["1024 byte", "16", "yaklaşık 100 ms + ACK gecikmeleri", "Orta boy test"],
        ["4096 byte", "64", "yaklaşık 400 ms + ACK gecikmeleri", "Kullanıcı alanının büyük kısmı"],
    ], [1.5, 1.2, 2.1, 1.7])
    p(doc, "Donanım kaynak tüketimi için sentez aracı raporu gereklidir. Repository içinde Gowin `.rpt` çıktısı bulunmadığı için LUT/Register/BRAM yüzdeleri bu rapora ölçülmüş değer olarak yazılmamıştır. Sunum öncesi Gowin EDA sentez raporundan `LUT`, `Register`, `BSRAM` ve maksimum frekans değerleri alınarak aşağıdaki tablo doldurulmalıdır. Bu ayrım, deneysel veriyi tahminle karıştırmamak için özellikle korunmuştur (PÇ7, PÇ13).")
    add_table(doc, ["Kaynak", "Ölçüm yeri", "Beklenen kullanım sınıfı", "Not"], [
        ["LUT", "Gowin synthesis utilization report", "PicoRV32 + UART + decode mantığı", "Rapor çıktısıyla doldurulacak"],
        ["Register", "Gowin synthesis utilization report", "CPU, UART ve dispatcher durumları", "Rapor çıktısıyla doldurulacak"],
        ["BRAM/BSRAM", "Gowin synthesis utilization report", "8 KiB program/data belleği", "bram_dp.v tarafından kullanılır"],
        ["Fmax", "Timing report", "27 MHz üstü yeterli", "Tang Nano 9K clock 27 MHz"],
    ], [1.4, 2.0, 1.7, 1.4])

    heading(doc, "4. Projenin Küresel, Toplumsal ve Ekonomik Etkileri", 1)
    heading(doc, "4.1. Sürdürülebilirlik ve Yeşil Bilişim", 2)
    p(doc, "RISC-V tabanlı küçük bir soft-core ve yazılım loader yaklaşımı, eğitim laboratuvarlarında büyük geliştirme kartları veya kapalı toolchain bağımlılıkları yerine düşük kaynaklı FPGA ortamı kullanılmasını sağlar. Daha küçük işlemci çekirdeği ve sade çevre birimleri, gereksiz donanım kaynaklarını azaltır. Programların UART üzerinden tekrar yüklenebilmesi, her deneme için bitstream üretme zorunluluğunu azaltarak geliştirme döngüsünde enerji ve zaman tasarrufu sağlar. Bu yönüyle sistem, SKA 7 temiz enerji ve SKA 13 iklim eylemi başlıklarıyla ilişkilendirilebilir (PÇ8).")
    heading(doc, "4.2. Ekonomik Sürdürülebilirlik ve Teknolojik Bağımsızlık", 2)
    p(doc, "Açık kaynak RISC-V ekosistemi, ticari mimarilere lisans bağımlılığını azaltır. Bu projede assembler, linker, object format ve loader protokolünün açıkça tasarlanması öğrencilerin yalnızca hazır araç kullanmasını değil, araç zincirinin iç mantığını da anlamasını sağlar. Bu bilgi birikimi yerli çip, gömülü sistem ve savunma elektroniği Ar-Ge çalışmalarında teknik bağımsızlığı destekler. Açık standartlara dayalı eğitim çıktısı, SKA 8 insana yakışır iş ve ekonomik büyüme ile SKA 9 sanayi, yenilikçilik ve altyapı hedefleriyle uyumludur (PÇ8, PÇ6).")
    heading(doc, "4.3. Fonksiyonel Güvenlik ve Sağlık", 2)
    p(doc, "Hata kontrollü loader mekanizması kritik gömülü sistemlerde doğrudan güvenlik konusudur. Tıbbi cihaz, otomotiv veya savunma sistemlerinde yanlış byte ile yüklenen bir firmware, fiziksel zarara yol açabilir. Bu projedeki checksum mekanizması eğitim ölçeğinde basit olsa da temel prensibi gösterir: doğrulanmamış veri belleğe yazılsa bile çalıştırılmamalıdır. START komutunda da checksum doğrulaması yapılması, program girişine yalnızca geçerli paketle dallanılmasını sağlar. Daha kritik sistemlerde CRC, imzalı firmware ve güvenli boot zinciri eklenmelidir (PÇ8, PÇ1).")
    heading(doc, "4.4. E-Atık Yönetimi ve Döngüsel Ekonomi", 2)
    p(doc, "FPGA üzerinde loader ile uzaktan veya seri porttan yeniden program yükleyebilmek, donanımı değiştirmeden yeni işlevlerin denenmesini sağlar. Bu yaklaşım eğitim kartlarının ömrünü uzatır ve her yazılım değişikliği için farklı donanım gereksinimini azaltır. Güncellenebilir sistemler, bakım ve iyileştirme yoluyla elektronik atığın azaltılmasına katkı sağlar. Bu etki SKA 12 sorumlu tüketim ve üretim hedefiyle ilişkilendirilebilir (PÇ8).")

    heading(doc, "5. Proje Yönetimi ve Takım Çalışması", 1)
    heading(doc, "5.1. Görev Dağılımı ve Sorumluluk Matrisi", 2)
    p(doc, "Proje çok modüllü olduğu için görevler doğal arayüz sınırlarına göre ayrılmıştır. Aşağıdaki RACI matrisi doldurulabilir takım şablonu olarak hazırlanmıştır; grup üyeleri A.XX, B.YY ve C.ZZ yerine gerçek isimleri yazmalıdır (PÇ12, PÇ13).")
    add_table(doc, ["İş paketi", "Responsible", "Accountable", "Consulted", "Informed"], [
        ["RV32I loader.s", "A.XX", "A.XX", "B.YY", "C.ZZ"],
        ["Python host loader", "B.YY", "B.YY", "A.XX", "C.ZZ"],
        ["UART/MMIO RTL entegrasyonu", "C.ZZ", "C.ZZ", "A.XX", "B.YY"],
        ["Assembler/linker build akışı", "A.XX", "B.YY", "C.ZZ", "Tüm ekip"],
        ["Test ve sunum videosu", "Tüm ekip", "C.ZZ", "Tüm ekip", "Tüm ekip"],
        ["Raporlama", "Tüm ekip", "A.XX", "B.YY, C.ZZ", "Tüm ekip"],
    ], [1.8, 1.2, 1.2, 1.2, 1.2])
    heading(doc, "5.2. Koordinasyon ve Sürüm Kontrol Yönetimi", 2)
    p(doc, "Kod, rapor, test ve FPGA dosyaları aynı Git repository içinde tutulmuştur. Arayüz standardı olarak bellek haritası, UART paket formatı ve linker çıktı dosyaları yazılı hale getirilmiştir. Bu yaklaşım, farklı ekip üyelerinin loader assembly, host script ve RTL tarafını aynı protokol üzerinde bağımsız geliştirmesine olanak verir. Entegrasyon krizlerini azaltmak için her değişiklikten sonra `make loader`, host script syntax kontrolü ve `go test ./...` çalıştırılmıştır (PÇ13).")

    heading(doc, "6. Bireysel Katkı Beyanı", 1)
    p(doc, "Bu bölüm her öğrenci tarafından ayrı ayrı doldurulup imzalanmalıdır. Aşağıdaki metin örneği, proje gerçeklerine göre düzenlenmelidir (PÇ12).")
    add_table(doc, ["Öğrenci", "Bağımsız tasarladığı modül", "Tek başına çözdüğü teknik problem", "İmza"], [
        ["A.XX", "RV32I software loader ve loader_link.toml", "UART paketlerini checksum ile doğrulayıp BRAM'e word hizalı yazma", ""],
        ["B.YY", "Python host loader ve paketleme akışı", ".bin/.mem dosyasını 32 bit word paketlerine dönüştürme", ""],
        ["C.ZZ", "UART MMIO ve PicoRV32 top entegrasyonu", "RX_READY/TX_BUSY polling yazmaçlarının CPU arayüzüne bağlanması", ""],
    ], [1.1, 2.0, 2.6, 0.8])

    heading(doc, "7. Kaynakça", 1)
    refs = [
        '[1] RISC-V International, "The RISC-V Instruction Set Manual, Volume I: Unprivileged ISA."',
        '[2] J. R. Levine, Linkers and Loaders, Morgan Kaufmann, 1999.',
        '[3] YosysHQ, "PicoRV32 - A Size-Optimized RISC-V CPU."',
        '[4] Nandland, "UART, Serial Port, RS-232 Interface" UART receiver/transmitter reference implementation.',
        '[5] P. Koopman, "32-Bit Cyclic Redundancy Codes for Internet Applications," DSN, 2002.',
        '[6] GNU Binutils Documentation, "Assembler and Linker Documentation."',
        '[7] IEEE, "IEEE Code of Ethics."',
        '[8] United Nations, "Sustainable Development Goals."',
        '[9] Gowin Semiconductor, "Tang Nano 9K / GW1NR FPGA Documentation."',
        '[10] Proje kaynak kodları: assembler/, linker/, gowin/loader.s, scripts/host_loader.py, docs/architecture.md.',
    ]
    for ref in refs:
        p(doc, ref)

    heading(doc, "Ek A - Değerlendirme Kriterleri Karşılama Matrisi", 1)
    add_table(doc, ["Kriter", "Raporda karşılandığı bölüm", "Kanıt", "PÇ"], [
        ["Literatür araştırması", "1.1, 1.2", "UART/SPI/JTAG, checksum/CRC karşılaştırması", "PÇ6"],
        ["Yöntem ve tasarım", "2.1, 2.2", "Toolchain arayüzleri, memory map, loader FSM akışı", "PÇ6"],
        ["Test tasarımı", "3.1", "Blink, counter, UART hello, checksum negatif testi", "PÇ7"],
        ["Analiz ve yorum", "3.2", "Loader boyutu, paket süresi, ölçüm ayrımı", "PÇ7"],
        ["Sürdürülebilirlik", "4.1-4.4", "Enerji, Ar-Ge maliyeti, güvenlik, e-atık analizi", "PÇ8"],
        ["Takım çalışması", "5.1, 5.2", "RACI matrisi ve Git/protokol koordinasyonu", "PÇ12/PÇ13"],
        ["Loader doğruluğu", "2.2, 3.2", "Checksum, ACK/ERR, START doğrulaması", "PÇ1"],
        ["FPGA örnek uygulama", "3.1", "LED ve UART gözlenebilir testleri", "PÇ12"],
        ["Rapor kalitesi", "Tüm rapor", "Courier New 10 pt, tablo/şekil/kod blokları", "PÇ13"],
    ], [1.4, 1.8, 2.4, 0.7])

    heading(doc, "Ek B - Kullanım Komutları", 1)
    add_code(doc, """# Loader boot image üretimi
make loader

# Örnek programların derlenmesi
make demos

# Programı UART üzerinden yükleme
python scripts\\host_loader.py build\\blink\\blink.bin -p COM5 --base 0x400 --entry 0x400

# Host script bağımlılığı
python -m pip install pyserial""")

    doc.save(DOCX)
    print(DOCX)


if __name__ == "__main__":
    build()
