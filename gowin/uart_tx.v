///////////////////////////////////////////////////////////////////////////////
// File:        uart_tx.v
// Source:      https://github.com/nandland/UART  (commit on main branch)
// Author:      Russell Merrick / nandland.com
// License:     MIT License
//
// Description:
//   This file contains the UART Transmitter. This transmitter is able
//   to transmit 8 bits of serial data, one start bit, one stop bit,
//   and no parity bit. When transmit is complete o_TX_Done will be
//   driven high for one clock cycle.
//
// Parameter:
//   CLKS_PER_BIT = (Frequency of i_Clock) / (Frequency of UART)
//   Example: 27 MHz Clock, 115200 baud UART
//            (27000000) / (115200) = 234
//
// Atıf notu (3. proje):
//   Bu dosya bilinçli olarak değiştirilmemiş hâliyle dahil edilmiştir.
//   Lisans: UART_LICENSE (MIT).
///////////////////////////////////////////////////////////////////////////////

module uart_tx
  #(parameter CLKS_PER_BIT = 234)
  (
   input       i_Clock,
   input       i_TX_DV,
   input [7:0] i_TX_Byte,
   output      o_TX_Active,
   output reg  o_TX_Serial,
   output      o_TX_Done
   );

  parameter s_IDLE         = 3'b000;
  parameter s_TX_START_BIT = 3'b001;
  parameter s_TX_DATA_BITS = 3'b010;
  parameter s_TX_STOP_BIT  = 3'b011;
  parameter s_CLEANUP      = 3'b100;

  reg [2:0]    r_SM_Main     = 0;
  reg [7:0]    r_Clk_Count   = 0;
  reg [2:0]    r_Bit_Index   = 0;
  reg [7:0]    r_TX_Data     = 0;
  reg          r_TX_Done     = 0;
  reg          r_TX_Active   = 0;

  always @(posedge i_Clock)
    begin

      case (r_SM_Main)
        s_IDLE :
          begin
            o_TX_Serial   <= 1'b1;         // Drive Line High for Idle
            r_TX_Done     <= 1'b0;
            r_Clk_Count   <= 0;
            r_Bit_Index   <= 0;

            if (i_TX_DV == 1'b1)
              begin
                r_TX_Active <= 1'b1;
                r_TX_Data   <= i_TX_Byte;
                r_SM_Main   <= s_TX_START_BIT;
              end
            else
              r_SM_Main <= s_IDLE;
          end // case: s_IDLE


        // Send out Start Bit. Start bit = 0
        s_TX_START_BIT :
          begin
            o_TX_Serial <= 1'b0;

            // Wait CLKS_PER_BIT-1 clock cycles for start bit to finish
            if (r_Clk_Count < CLKS_PER_BIT-1)
              begin
                r_Clk_Count <= r_Clk_Count + 1;
                r_SM_Main   <= s_TX_START_BIT;
              end
            else
              begin
                r_Clk_Count <= 0;
                r_SM_Main   <= s_TX_DATA_BITS;
              end
          end // case: s_TX_START_BIT


        // Wait CLKS_PER_BIT-1 clock cycles for data bits to finish
        s_TX_DATA_BITS :
          begin
            o_TX_Serial <= r_TX_Data[r_Bit_Index];

            if (r_Clk_Count < CLKS_PER_BIT-1)
              begin
                r_Clk_Count <= r_Clk_Count + 1;
                r_SM_Main   <= s_TX_DATA_BITS;
              end
            else
              begin
                r_Clk_Count <= 0;

                // Check if we have sent out all bits
                if (r_Bit_Index < 7)
                  begin
                    r_Bit_Index <= r_Bit_Index + 1;
                    r_SM_Main   <= s_TX_DATA_BITS;
                  end
                else
                  begin
                    r_Bit_Index <= 0;
                    r_SM_Main   <= s_TX_STOP_BIT;
                  end
              end
          end // case: s_TX_DATA_BITS


        // Send out Stop bit.  Stop bit = 1
        s_TX_STOP_BIT :
          begin
            o_TX_Serial <= 1'b1;

            // Wait CLKS_PER_BIT-1 clock cycles for Stop bit to finish
            if (r_Clk_Count < CLKS_PER_BIT-1)
              begin
                r_Clk_Count <= r_Clk_Count + 1;
                r_SM_Main   <= s_TX_STOP_BIT;
              end
            else
              begin
                r_TX_Done   <= 1'b1;
                r_Clk_Count <= 0;
                r_SM_Main   <= s_CLEANUP;
                r_TX_Active <= 1'b0;
              end
          end // case: s_TX_STOP_BIT


        // Stay here 1 clock
        s_CLEANUP :
          begin
            r_TX_Done <= 1'b1;
            r_SM_Main <= s_IDLE;
          end


        default :
          r_SM_Main <= s_IDLE;

      endcase
    end

  assign o_TX_Active = r_TX_Active;
  assign o_TX_Done   = r_TX_Done;

endmodule
