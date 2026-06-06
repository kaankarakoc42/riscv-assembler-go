import sys
import subprocess
import time

try:
    import serial.tools.list_ports
except ImportError:
    print("[OTOMATİK] 'pyserial' kütüphanesi bulunamadı. Kuruluyor...")
    subprocess.check_call([sys.executable, "-m", "pip", "install", "pyserial"])
    import serial.tools.list_ports

def find_tang_nano_hardware():
    """
    Sistemdeki portları donanımsal üretici kimliklerine (VID/PID) göre tarar.
    Tang Nano 9K üzerindeki FTDI çipinin standart üretici kimliği: VID_0403
    """
    ports = list(serial.tools.list_ports.comports())
    tang_ports = []

    for port in ports:
        hwid = port.hwid.upper()
        desc = port.description.upper()
        device = port.device
        
        # FTDI Çiplerinin evrensel üretici kodu 0403'tür (Gowin Programmer da bunu baz alır)
        if "VID_0403" in hwid or "FTDI" in hwid or "PID_6010" in hwid:
            tang_ports.append(device)
            continue
            
        # Eğer Windows sürücü adını generic yaptıysa ikinci bir emniyet filtresi
        if "USB SERIAL" in desc or "DUAL RS232" in desc:
            tang_ports.append(device)

    if not tang_ports:
        # Sistemdeki tüm portları logla (Debug kolaylığı için)
        if ports:
            print("[DEBUG] Tang Nano bulunamadı ama sistemde şu portlar var:")
            for p in ports:
                print(f"  -> {p.device}: {p.description} [HWID: {p.hwid}]")
        return None

    # Portları sayısal büyüklüklerine göre sırala (Örn: COM3, COM4)
    try:
        tang_ports.sort(key=lambda x: int(''.join(filter(str.isdigit, x))))
    except ValueError:
        tang_ports.sort()

    print(f"[OTOMATİK] Donanımsal Olarak Eşleşen Portlar: {tang_ports}")

    if len(tang_ports) >= 2:
        # Altın Kural: Büyük port numarası her zaman UART Interface B hattıdır.
        selected_port = tang_ports[-1]
        print(f"[OTOMATİK] JTAG Yükleme Portu (A): {tang_ports[0]}")
        print(f"[OTOMATİK] Yazılım Loader Portu (B): {selected_port} <= SEÇİLDİ")
        return selected_port
    else:
        # Tek port döndüyse doğrudan onu seç
        return tang_ports[0]

def main():
    if len(sys.argv) < 2:
        print("Kullanım: python auto_host_loader.py <hex_dosya_yolu>")
        print("Örnek:   python auto_host_loader.py ..\\src\\program.hex")
        sys.exit(1)
        
    hex_file = sys.argv[1]
    
    print("[OTOMATİK] Donanım kimliği üzerinden Tang Nano 9K aranıyor...")
    uart_port = find_tang_nano_hardware()
    
    if not uart_port:
        print("\n[HATA] Bilgisayara bağlı Tang Nano 9K donanımı tespit edilemedi!")
        print("1. Kartın USB kablosunun bilgisayara tam oturduğundan emin olun.")
        print("2. Aygıt Yöneticisi'nde 'Bağlantı Noktaları (COM ve LPT)' sekmesi görünüyor mu kontrol edin.")
        sys.exit(1)
        
    # Mevcut host_loader.py betiğini tetikle
    cmd = [
        sys.executable, "host_loader.py", 
        "-p", uart_port, 
        "-b", "115200", 
        hex_file
    ]
    
    print(f"[OTOMATİK] Komut Çalıştırılıyor: {' '.join(cmd)}\n")
    
    process = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    
    while True:
        output = process.stdout.readline()
        if output == '' and process.poll() is not None:
            break
        if output:
            print(output.strip())
            
    if process.poll() == 0:
        print("\n[OTOMATİK] Yükleme işlemi başarıyla bitti.")
    else:
        print(f"\n[HATA] host_loader.py yürütme hatası kodu: {process.poll()}")

if __name__ == "__main__":
    main()