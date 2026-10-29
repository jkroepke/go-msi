# MSIDB-12 / transform-streams
built base.msi sha256 5900655773050cfaa8e2397966f00c1f03afb90f1ddc039c92e85d722ac51174
change: MstKeep a: Text alpha -> ALPHA (one string column)
change: MstKeep b: Num 2 -> NULL, Big 200000 -> -5 (two integer columns)
change: MstKeep c: deleted
change: MstKeep d: inserted
change: MstTwoKey x/2: Val two -> TWO; x/3 inserted
change: MstWide w01: C18 w18 -> W18 (column 18, past the mask's 16 bits)
change: MstWide v02: C05 changed (column 5 of a 20-column table)
change: MstWide v03: C05 and C18 changed
change: MstWide v04: C16 changed
change: MstWide v05: C17 changed
change: MstWide v06: inserted (20 columns)
change: MstWide v07: deleted
change: MstKeep e: Text epsilon -> NULL
change: MstAlter: column Extra added, row k gets Extra 'e'
change: MstNew: table created with one row
change: MstOld: table only in the reference, dropped
change: Property: ProductName changed, MstProp inserted
## plain.mst (error conditions 0x0, validation 0x0) sha256 4bbbc02443578da1d7b93f4b2e464ec627cd380061bfe6980b621bcdbbaa4c2f
  PID 1 VT 2 int 1252 (0x4e4) text "" filetime [0 0]
  PID 2 VT 30 int 0 (0x0) text "Installation Database" filetime [0 0]
  PID 3 VT 30 int 0 (0x0) text "MsiSpec Test msidb12" filetime [0 0]
  PID 4 VT 30 int 0 (0x0) text "msitest" filetime [0 0]
  PID 5 VT 30 int 0 (0x0) text "" filetime [0 0]
  PID 6 VT 30 int 0 (0x0) text "" filetime [0 0]
  PID 7 VT 30 int 0 (0x0) text "x64;1033" filetime [0 0]
  PID 8 VT 30 int 0 (0x0) text "x64;1033" filetime [0 0]
  PID 9 VT 30 int 0 (0x0) text "{7D55CFA7-6A52-4B15-A841-5A705AF62AD6}1.0.0;{7D55CFA7-6A52-4B15-A841-5A705AF62AD6}1.0.0;{5E7A1C2B-0D3F-4A6E-9B8C-7F1E2D3C4B5A}" filetime [0 0]
  PID 12 VT 64 int 0 (0x0) text "" filetime [2457927680 31226545]
  PID 14 VT 3 int 200 (0xc8) text "" filetime [0 0]
  PID 16 VT 3 int 0 (0x0) text "" filetime [0 0]
  PID 18 VT 30 int 0 (0x0) text "msitest" filetime [0 0]
  PID 19 VT 3 int 2 (0x2) text "" filetime [0 0]
file plain.mst: 20480 bytes, CFB major 4, sector 4096
entry SID 0 parent -1 type 5 (root) name "Root Entry" decoded "Root Entry" clsid 82100c0000000000c000000000000046 size 1664
entry SID 1 parent 0 type 2 (table) name "䡀䕙䓲䕨䜷" decoded "Property" clsid 00000000000000000000000000000000 size 12
  hex 020001000200010203000400
entry SID 2 parent 0 type 2 (table) name "䡀䖖㴷䈨䠳" decoded "MstKeep" clsid 00000000000000000000000000000000 size 38
  hex 080005000600060007000000fbffff7f010408000480801a0680090008000a00000000000b00
entry SID 3 parent 0 type 2 (table) name "䡀䖖㽷䒺䈔䠼" decoded "MstTwoKey" clsid 00000000000000000000000000000000 size 16
  hex 04000c0002800d0001030c0003800e00
entry SID 4 parent 0 type 2 (table) name "䡀䖖䀷䇬䠨" decoded "MstWide" clsid 00000000000000000000000000000000 size 94
  hex 0700000002000f00100010000000110012001000020013001400150000800000160017000000010018001900011400001a001b001c001d001e001f0020002100220023002400250026002700280029002a002b002c002d00000000002e00
entry SID 5 parent 0 type 2 (table) name "䡀㬿䏲䐸䖱" decoded "_Columns" clsid 00000000000000000000000000000000 size 30
  hex 01042f0002803000149d010432000000330048ad01043200000034000295
entry SID 6 parent 0 type 2 (table) name "䡀䖖㪷䗯䕨" decoded "MstAlter" clsid 00000000000000000000000000000000 size 6
  hex 020031000a00
entry SID 7 parent 0 type 2 (table) name "䡀㽿䅤䈯䠶" decoded "_Tables" clsid 00000000000000000000000000000000 size 8
  hex 0101320000003600
entry SID 8 parent 0 type 2 (table) name "䡀䖖㷷䚨" decoded "MstNew" clsid 00000000000000000000000000000000 size 6
  hex 010235000780
entry SID 9 parent 0 type 2 (table) name "䡀㼿䕷䑬㹪䒲䠯" decoded "_StringPool" clsid 00000000000000000000000000000000 size 220
  hex 000000000b0001001c00010007000100050001000100010005000100010001000100010005000100010002000100010001000200030001000500010003000100030001000300010006000100030001000600010006000100030001000600010003000100060001000300010006000100060001000600010006000100060001000600010006000100060001000600010006000100060001000600010006000100060001000600010006000100060001000600010006000100030001000800010005000100010001000600030003000100030001000100010006000100
entry SID 10 parent 0 type 2 (table) name "䡀㼿䕷䑬㭪䗤䠤" decoded "_StringData" clsid 00000000000000000000000000000000 size 276
  hex 50726f647563744e616d654d7369537065632054657374206d736964623132206368616e6765644d737450726f70616464656461414c504841626464656c746165637854574f74687265657730315731387630325830325f30357630335830335f30355830335f31387630345830345f31367630355830355f31377630367630365f30327630365f30337630365f30347630365f30357630365f30367630365f30377630365f30387630365f30397630365f31307630365f31317630365f31327630365f31337630365f31347630365f31357630365f31367630365f31377630365f31387630365f31397630365f32307630374d7374416c74657245787472616b4d73744e65774b65794e756d6e4d73744f6c64
entry SID 11 parent 0 type 2 (stream) name "\x05SummaryInformation" decoded "\x05SummaryInformation" clsid 00000000000000000000000000000000 size 508
  hex feff00000a0002000000000000000000000000000000000001000000e0859ff2f94f6810ab9108002b27b3d930000000cc0100000e0000000100000078000000020000008000000003000000a000000004000000c000000005000000d000000006000000dc0000000c000000e800000012000000f40000001300000004010000070000000c010000080000002001000009000000340100000e000000bc01000010000000c401000002000000e40400001e00000016000000496e7374616c6c6174696f6e2044617461626173650000001e000000150000004d7369537065632054657374206d736964623132000000001e000000080000006d736974657374001e00000001000000000000001e00000001000000000000004000000000008192b17adc011e000000080000006d7369746573740003000000020000001e000000090000007836343b31303333000000001e000000090000007836343b31303333000000001e0000007f0000007b37443535434641372d364135322d344231352d413834312d3541373035414636324144367d312e302e303b7b37443535434641372d364135322d344231352d413834312d3541373035414636324144367d312e302e303b7b35453741314332422d304433462d344136452d394238432d3746314532443343344235417d000003000000c80000000300000000000000
entry SID 12 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 13 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 14 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 15 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 16 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 17 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 18 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 19 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 20 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 21 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 22 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 23 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 24 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 25 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 26 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 27 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 28 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 29 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 30 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 31 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
_StringPool header 0x00000000: code page 0, long references false
  string 1 (refcount 1): "ProductName"
  string 2 (refcount 1): "MsiSpec Test msidb12 changed"
  string 3 (refcount 1): "MstProp"
  string 4 (refcount 1): "added"
  string 5 (refcount 1): "a"
  string 6 (refcount 1): "ALPHA"
  string 7 (refcount 1): "b"
  string 8 (refcount 1): "d"
  string 9 (refcount 1): "delta"
  string 10 (refcount 2): "e"
  string 11 (refcount 1): "c"
  string 12 (refcount 2): "x"
  string 13 (refcount 1): "TWO"
  string 14 (refcount 1): "three"
  string 15 (refcount 1): "w01"
  string 16 (refcount 1): "W18"
  string 17 (refcount 1): "v02"
  string 18 (refcount 1): "X02_05"
  string 19 (refcount 1): "v03"
  string 20 (refcount 1): "X03_05"
  string 21 (refcount 1): "X03_18"
  string 22 (refcount 1): "v04"
  string 23 (refcount 1): "X04_16"
  string 24 (refcount 1): "v05"
  string 25 (refcount 1): "X05_17"
  string 26 (refcount 1): "v06"
  string 27 (refcount 1): "v06_02"
  string 28 (refcount 1): "v06_03"
  string 29 (refcount 1): "v06_04"
  string 30 (refcount 1): "v06_05"
  string 31 (refcount 1): "v06_06"
  string 32 (refcount 1): "v06_07"
  string 33 (refcount 1): "v06_08"
  string 34 (refcount 1): "v06_09"
  string 35 (refcount 1): "v06_10"
  string 36 (refcount 1): "v06_11"
  string 37 (refcount 1): "v06_12"
  string 38 (refcount 1): "v06_13"
  string 39 (refcount 1): "v06_14"
  string 40 (refcount 1): "v06_15"
  string 41 (refcount 1): "v06_16"
  string 42 (refcount 1): "v06_17"
  string 43 (refcount 1): "v06_18"
  string 44 (refcount 1): "v06_19"
  string 45 (refcount 1): "v06_20"
  string 46 (refcount 1): "v07"
  string 47 (refcount 1): "MstAlter"
  string 48 (refcount 1): "Extra"
  string 49 (refcount 1): "k"
  string 50 (refcount 3): "MstNew"
  string 51 (refcount 1): "Key"
  string 52 (refcount 1): "Num"
  string 53 (refcount 1): "n"
  string 54 (refcount 1): "MstOld"
  view ["Property" "Value" "ProductName" "MsiSpec Test msidb12 changed" "MsiSpec Test msidb12"]
  view ["Property" "Value" "MstProp" "added" "\x00NULL"]
  view ["Property" "INSERT" "MstProp" "\x00NULL" "\x00NULL"]
  view ["MstKeep" "Num" "b" "\x00NULL" "2"]
  view ["MstKeep" "Num" "d" "4" "\x00NULL"]
  view ["MstKeep" "Big" "b" "-5" "200000"]
  view ["MstKeep" "Big" "d" "400000" "\x00NULL"]
  view ["MstKeep" "Text" "a" "ALPHA" "alpha"]
  view ["MstKeep" "Text" "e" "\x00NULL" "epsilon"]
  view ["MstKeep" "Text" "d" "delta" "\x00NULL"]
  view ["MstKeep" "INSERT" "d" "\x00NULL" "\x00NULL"]
  view ["MstKeep" "DELETE" "c" "\x00NULL" "\x00NULL"]
  view ["MstTwoKey" "Val" "x\t2" "TWO" "two"]
  view ["MstTwoKey" "Val" "x\t3" "three" "\x00NULL"]
  view ["MstTwoKey" "INSERT" "x\t3" "\x00NULL" "\x00NULL"]
  view ["MstWide" "C02" "v06" "v06_02" "\x00NULL"]
  view ["MstWide" "C03" "v06" "v06_03" "\x00NULL"]
  view ["MstWide" "C04" "v06" "v06_04" "\x00NULL"]
  view ["MstWide" "C05" "v02" "X02_05" "v02_05"]
  view ["MstWide" "C05" "v03" "X03_05" "v03_05"]
  view ["MstWide" "C05" "v06" "v06_05" "\x00NULL"]
  view ["MstWide" "C06" "v06" "v06_06" "\x00NULL"]
  view ["MstWide" "C07" "v06" "v06_07" "\x00NULL"]
  view ["MstWide" "C08" "v06" "v06_08" "\x00NULL"]
  view ["MstWide" "C09" "v06" "v06_09" "\x00NULL"]
  view ["MstWide" "C10" "v06" "v06_10" "\x00NULL"]
  view ["MstWide" "C11" "v06" "v06_11" "\x00NULL"]
  view ["MstWide" "C12" "v06" "v06_12" "\x00NULL"]
  view ["MstWide" "C13" "v06" "v06_13" "\x00NULL"]
  view ["MstWide" "C14" "v06" "v06_14" "\x00NULL"]
  view ["MstWide" "C15" "v06" "v06_15" "\x00NULL"]
  view ["MstWide" "C16" "v04" "X04_16" "v04_16"]
  view ["MstWide" "C16" "v06" "v06_16" "\x00NULL"]
  view ["MstWide" "C17" "v05" "X05_17" "v05_17"]
  view ["MstWide" "C17" "v06" "v06_17" "\x00NULL"]
  view ["MstWide" "C18" "w01" "W18" "w18"]
  view ["MstWide" "C18" "v03" "X03_18" "v03_18"]
  view ["MstWide" "C18" "v06" "v06_18" "\x00NULL"]
  view ["MstWide" "C19" "v06" "v06_19" "\x00NULL"]
  view ["MstWide" "C20" "v06" "v06_20" "\x00NULL"]
  view ["MstWide" "INSERT" "v06" "\x00NULL" "\x00NULL"]
  view ["MstWide" "DELETE" "v07" "\x00NULL" "\x00NULL"]
  view ["MstAlter" "Extra" "\x00NULL" "7444" "2"]
  view ["MstAlter" "Extra" "k" "e" "\x00NULL"]
  view ["MstOld" "DROP" "\x00NULL" "\x00NULL" "\x00NULL"]
  view ["MstNew" "Key" "\x00NULL" "11592" "1"]
  view ["MstNew" "Num" "\x00NULL" "5378" "2"]
  view ["MstNew" "Num" "n" "7" "\x00NULL"]
  view ["MstNew" "CREATE" "\x00NULL" "\x00NULL" "\x00NULL"]
  view ["MstNew" "INSERT" "n" "\x00NULL" "\x00NULL"]
## flags.mst (error conditions 0x3f, validation 0x90b) sha256 3dc4e0459ddb7725e18d2d7919752187eaa39cbff5e687b8375216e1f4e770cb
  PID 1 VT 2 int 1252 (0x4e4) text "" filetime [0 0]
  PID 2 VT 30 int 0 (0x0) text "Installation Database" filetime [0 0]
  PID 3 VT 30 int 0 (0x0) text "MsiSpec Test msidb12" filetime [0 0]
  PID 4 VT 30 int 0 (0x0) text "msitest" filetime [0 0]
  PID 5 VT 30 int 0 (0x0) text "" filetime [0 0]
  PID 6 VT 30 int 0 (0x0) text "" filetime [0 0]
  PID 7 VT 30 int 0 (0x0) text "x64;1033" filetime [0 0]
  PID 8 VT 30 int 0 (0x0) text "x64;1033" filetime [0 0]
  PID 9 VT 30 int 0 (0x0) text "{7D55CFA7-6A52-4B15-A841-5A705AF62AD6}1.0.0;{7D55CFA7-6A52-4B15-A841-5A705AF62AD6}1.0.0;{5E7A1C2B-0D3F-4A6E-9B8C-7F1E2D3C4B5A}" filetime [0 0]
  PID 12 VT 64 int 0 (0x0) text "" filetime [2457927680 31226545]
  PID 14 VT 3 int 200 (0xc8) text "" filetime [0 0]
  PID 16 VT 3 int 151715903 (0x90b003f) text "" filetime [0 0]
  PID 18 VT 30 int 0 (0x0) text "msitest" filetime [0 0]
  PID 19 VT 3 int 2 (0x2) text "" filetime [0 0]
## cp1252: variant 0	_ForceCodepage, text "Zurück"
  PID 1 VT 2 int 1252 (0x4e4) text "" filetime [0 0]
  PID 2 VT 30 int 0 (0x0) text "Installation Database" filetime [0 0]
  PID 3 VT 30 int 0 (0x0) text "MsiSpec Test msidb12" filetime [0 0]
  PID 4 VT 30 int 0 (0x0) text "msitest" filetime [0 0]
  PID 5 VT 30 int 0 (0x0) text "" filetime [0 0]
  PID 6 VT 30 int 0 (0x0) text "" filetime [0 0]
  PID 7 VT 30 int 0 (0x0) text "x64;1033" filetime [0 0]
  PID 8 VT 30 int 0 (0x0) text "x64;1033" filetime [0 0]
  PID 9 VT 30 int 0 (0x0) text "{7D55CFA7-6A52-4B15-A841-5A705AF62AD6}1.0.0;{7D55CFA7-6A52-4B15-A841-5A705AF62AD6}1.0.0;{5E7A1C2B-0D3F-4A6E-9B8C-7F1E2D3C4B5A}" filetime [0 0]
  PID 12 VT 64 int 0 (0x0) text "" filetime [2457927680 31226545]
  PID 14 VT 3 int 200 (0xc8) text "" filetime [0 0]
  PID 16 VT 3 int 0 (0x0) text "" filetime [0 0]
  PID 18 VT 30 int 0 (0x0) text "msitest" filetime [0 0]
  PID 19 VT 3 int 2 (0x2) text "" filetime [0 0]
file cp1252.mst: 20480 bytes, CFB major 4, sector 4096
entry SID 0 parent -1 type 5 (root) name "Root Entry" decoded "Root Entry" clsid 82100c0000000000c000000000000046 size 704
entry SID 1 parent 0 type 2 (table) name "䡀䕙䓲䕨䜷" decoded "Property" clsid 00000000000000000000000000000000 size 6
  hex 010201000200
entry SID 2 parent 0 type 2 (table) name "䡀㼿䕷䑬㹪䒲䠯" decoded "_StringPool" clsid 00000000000000000000000000000000 size 12
  hex 000000000700010006000100
entry SID 3 parent 0 type 2 (table) name "䡀㼿䕷䑬㭪䗤䠤" decoded "_StringData" clsid 00000000000000000000000000000000 size 13
  hex 4d7374546578745a7572fc636b
entry SID 4 parent 0 type 2 (stream) name "\x05SummaryInformation" decoded "\x05SummaryInformation" clsid 00000000000000000000000000000000 size 508
  hex feff00000a0002000000000000000000000000000000000001000000e0859ff2f94f6810ab9108002b27b3d930000000cc0100000e0000000100000078000000020000008000000003000000a000000004000000c000000005000000d000000006000000dc0000000c000000e800000012000000f40000001300000004010000070000000c010000080000002001000009000000340100000e000000bc01000010000000c401000002000000e40400001e00000016000000496e7374616c6c6174696f6e2044617461626173650000001e000000150000004d7369537065632054657374206d736964623132000000001e000000080000006d736974657374001e00000001000000000000001e00000001000000000000004000000000008192b17adc011e000000080000006d7369746573740003000000020000001e000000090000007836343b31303333000000001e000000090000007836343b31303333000000001e0000007f0000007b37443535434641372d364135322d344231352d413834312d3541373035414636324144367d312e302e303b7b37443535434641372d364135322d344231352d413834312d3541373035414636324144367d312e302e303b7b35453741314332422d304433462d344136452d394238432d3746314532443343344235417d000003000000c80000000300000000000000
entry SID 5 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 6 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 7 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 8 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 9 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 10 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 11 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 12 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 13 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 14 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 15 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 16 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 17 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 18 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 19 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 20 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 21 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 22 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 23 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 24 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 25 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 26 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 27 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 28 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 29 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 30 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 31 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
_StringPool header 0x00000000: code page 0, long references false
  string 1 (refcount 1): "MstText"
  string 2 (refcount 1): "Zur\xfcck"
cp1252: applied to the 1252 reference: 0	_ForceCodepage, MstText [["Zurück"]] (<nil>), equal true
## cp65001: variant 65001	_ForceCodepage, text "Назад Zurück 日本"
  PID 1 VT 2 int 1252 (0x4e4) text "" filetime [0 0]
  PID 2 VT 30 int 0 (0x0) text "Installation Database" filetime [0 0]
  PID 3 VT 30 int 0 (0x0) text "MsiSpec Test msidb12" filetime [0 0]
  PID 4 VT 30 int 0 (0x0) text "msitest" filetime [0 0]
  PID 5 VT 30 int 0 (0x0) text "" filetime [0 0]
  PID 6 VT 30 int 0 (0x0) text "" filetime [0 0]
  PID 7 VT 30 int 0 (0x0) text "x64;1033" filetime [0 0]
  PID 8 VT 30 int 0 (0x0) text "x64;1033" filetime [0 0]
  PID 9 VT 30 int 0 (0x0) text "{7D55CFA7-6A52-4B15-A841-5A705AF62AD6}1.0.0;{7D55CFA7-6A52-4B15-A841-5A705AF62AD6}1.0.0;{5E7A1C2B-0D3F-4A6E-9B8C-7F1E2D3C4B5A}" filetime [0 0]
  PID 12 VT 64 int 0 (0x0) text "" filetime [2457927680 31226545]
  PID 14 VT 3 int 200 (0xc8) text "" filetime [0 0]
  PID 16 VT 3 int 0 (0x0) text "" filetime [0 0]
  PID 18 VT 30 int 0 (0x0) text "msitest" filetime [0 0]
  PID 19 VT 3 int 2 (0x2) text "" filetime [0 0]
file cp65001.mst: 20480 bytes, CFB major 4, sector 4096
entry SID 0 parent -1 type 5 (root) name "Root Entry" decoded "Root Entry" clsid 82100c0000000000c000000000000046 size 704
entry SID 1 parent 0 type 2 (table) name "䡀䕙䓲䕨䜷" decoded "Property" clsid 00000000000000000000000000000000 size 6
  hex 010201000200
entry SID 2 parent 0 type 2 (table) name "䡀㼿䕷䑬㹪䒲䠯" decoded "_StringPool" clsid 00000000000000000000000000000000 size 12
  hex e9fd00000700010019000180
entry SID 3 parent 0 type 2 (table) name "䡀㼿䕷䑬㭪䗤䠤" decoded "_StringData" clsid 00000000000000000000000000000000 size 32
  hex 4d737454657874d09dd0b0d0b7d0b0d0b4205a7572c3bc636b20e697a5e69cac
entry SID 4 parent 0 type 2 (stream) name "\x05SummaryInformation" decoded "\x05SummaryInformation" clsid 00000000000000000000000000000000 size 508
  hex feff00000a0002000000000000000000000000000000000001000000e0859ff2f94f6810ab9108002b27b3d930000000cc0100000e0000000100000078000000020000008000000003000000a000000004000000c000000005000000d000000006000000dc0000000c000000e800000012000000f40000001300000004010000070000000c010000080000002001000009000000340100000e000000bc01000010000000c401000002000000e40400001e00000016000000496e7374616c6c6174696f6e2044617461626173650000001e000000150000004d7369537065632054657374206d736964623132000000001e000000080000006d736974657374001e00000001000000000000001e00000001000000000000004000000000008192b17adc011e000000080000006d7369746573740003000000020000001e000000090000007836343b31303333000000001e000000090000007836343b31303333000000001e0000007f0000007b37443535434641372d364135322d344231352d413834312d3541373035414636324144367d312e302e303b7b37443535434641372d364135322d344231352d413834312d3541373035414636324144367d312e302e303b7b35453741314332422d304433462d344136452d394238432d3746314532443343344235417d000003000000c80000000300000000000000
entry SID 5 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 6 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 7 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 8 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 9 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 10 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 11 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 12 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 13 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 14 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 15 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 16 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 17 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 18 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 19 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 20 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 21 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 22 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 23 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 24 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 25 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 26 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 27 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 28 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 29 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 30 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
entry SID 31 parent -1 type 0 (stream) name "" decoded "" clsid 00000000000000000000000000000000 size 0
_StringPool header 0x0000fde9: code page 65001, long references false
  string 1 (refcount 1): "MstText"
  string 2 (refcount 32769): "Назад Zurück 日本"
cp65001: applied to the 1252 reference: 65001	_ForceCodepage, MstText [["Назад Zurück 日本"]] (<nil>), equal true
FAIL
