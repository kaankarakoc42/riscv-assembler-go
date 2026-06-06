///////////////////////////////////////////////////////////////////////////////
// File:        uart_rx.v
// Source:      https://github.com/nandland/UART  (commit on main branch)
// Author:      Russell Merrick / nandland.com
// License:     MIT License
//
// Description:
//   This file contains the UART Receiver. This receiver is able to
//   receive 8 bits of serial data, one start bit, one stop bit, and no
//   parity bit. When receive is complete o_RX_DV will be driven high
//   for one clock cycle.
//
// Parameter:
//   CLKS_PER_BIT = (Frequency of i_Clk) / (Frequency of UART)
//   Example: 27 MHz Clock, 115200 baud UART
//            (27000000) / (115200) = 234
//
// Atıf notu (3. proje):
//   Bu dosya bilinçli olarak değiştirilmemiş hâliyle dahil edilmiştir.
//   Lisans: UART_LICENSE (MIT). Detaylar için reponun LICENSE dosyasına
//   bakınız.
///////////////////////////////////////////////////////////////////////////////

module uart_rx
  #(parameter CLKS_PER_BIT = 234)
  (
   input        i_Clock,
   input        i_RX_Serial,
   output       o_RX_DV,
   output [7:0] o_RX_Byte
   );

  parameter s_IDLE         = 3'b000;
  parameter s_RX_START_BIT = 3'b001;
  parameter s_RX_DATA_BITS = 3'b010;
  parameter s_RX_STOP_BIT  = 3'b011;
  parameter s_CLEANUP      = 3'b100;

  reg           r_RX_Data_R = 1'b1;
  reg           r_RX_Data   = 1'b1;

  reg [7:0]     r_Clk_Count = 0;
  reg [2:0]     r_Bit_Index = 0; //8 bits total
  reg [7:0]     r_RX_Byte   = 0;
  reg           r_RX_DV     = 0;
  reg [2:0]     r_SM_Main   = 0;

  // Purpose: Double-register the incoming data.
  // This allows it to be used in the UART RX Clock Domain.
  // (It removes problems caused by metastability)
  always @(posedge i_Clock)
    begin
      r_RX_Data_R <= i_RX_Serial;
      r_RX_Data   <= r_RX_Data_R;
    end


  // Purpose: Control RX state machine
  always @(posedge i_Clock)
    begin

      case (r_SM_Main)
        s_IDLE :
          begin
            r_RX_DV     <= 1'b0;
            r_Clk_Count <= 0;
            r_Bit_Index <= 0;

            if (r_RX_Data == 1'b0)          // Start bit detected
              r_SM_Main <= s_RX_START_BIT;
            else
              r_SM_Main <= s_IDLE;
          end

        // Check middle of start bit to make sure it's still low
        s_RX_START_BIT :
          begin
            if (r_Clk_Count == (CLKS_PER_BIT-1)/2)
              begin
                if (r_RX_Data == 1'b0)
                  begin
                    r_Clk_Count <= 0;  // reset counter, found the middle
                    r_SM_Main   <= s_RX_DATA_BITS;
                  end
                else
                  r_SM_Main <= s_IDLE;
              end
            else
              begin
                r_Clk_Count <= r_Clk_Count + 1;
                r_SM_Main   <= s_RX_START_BIT;
              end
          end // case: s_RX_START_BIT


        // Wait CLKS_PER_BIT-1 clock cycles to sample serial data
        s_RX_DATA_BITS :
          begin
            if (r_Clk_Count < CLKS_PER_BIT-1)
              begin
                r_Clk_Count <= r_Clk_Count + 1;
                r_SM_Main   <= s_RX_DATA_BITS;
              end
            else
              begin
                r_Clk_Count          <= 0;
                r_RX_Byte[r_Bit_Index] <= r_RX_Data;

                // Check if we have received all bits
                if (r_Bit_Index < 7)
                  begin
                    r_Bit_Index <= r_Bit_Index + 1;
                    r_SM_Main   <= s_RX_DATA_BITS;
                  end
                else
                  begin
                    r_Bit_Index <= 0;
                    r_SM_Main   <= s_RX_STOP_BIT;
                  end
              end
          end // case: s_RX_DATA_BITS


        // Receive Stop bit.  Stop bit = 1
        s_RX_STOP_BIT :
          begin
            // Wait CLKS_PER_BIT-1 clock cycles for Stop bit to finish
            if (r_Clk_Count < CLKS_PER_BIT-1)
              begin
                r_Clk_Count <= r_Clk_Count + 1;
                r_SM_Main   <= s_RX_STOP_BIT;
              end
            else
              begin
                r_RX_DV     <= 1'b1;
                r_Clk_Count <= 0;
                r_SM_Main   <= s_CLEANUP;
              end
          end // case: s_RX_STOP_BIT


        // Stay here 1 clock
        s_CLEANUP :
          begin
            r_SM_Main <= s_IDLE;
            r_RX_DV   <= 1'b0;
          end


        default :
          r_SM_Main <= s_IDLE;

      endcase
    end

  assign o_RX_DV   = r_RX_DV;
  assign o_RX_Byte = r_RX_Byte;

endmodule // uart_rx
