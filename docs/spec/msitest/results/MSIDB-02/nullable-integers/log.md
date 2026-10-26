# MSIDB-02 / nullable-integers
create: <nil>
insert row 1: <nil>
insert row 2: <nil>
insert row 3: <nil>
insert row 4: <nil>
insert row 5: <nil>
INSERT INTO `T` (`K`,`S`) VALUES (6, -32768) -> <nil>
INSERT INTO `T` (`K`,`L`) VALUES (7, -2147483648) -> <nil>
INSERT INTO `T` (`K`,`S`) VALUES (8, 40000) -> MsiViewExecute: 1627 (1: 2210 2: results\MSIDB-02\nullable-integers\ints.msi 3: 1 )
commit <nil>
native: ["1" "-32767" "-2147483647" "x"]
native: ["2" "32767" "2147483647" "x"]
native: ["3" "0" "0" "x"]
native: ["4" "-1" "-1" "x"]
native: ["5" "\x00NULL" "\x00NULL" "\x00NULL"]
native: ["6" "\x00NULL" "\x00NULL" "\x00NULL"]
native: ["7" "\x00NULL" "\x00NULL" "\x00NULL"]
raw: K=0x8001(1) S=0x1(-32767) L=0x1(-2147483647) C=0xb(x) B=0x0()
raw: K=0x8002(2) S=0xffff(32767) L=0xffffffff(2147483647) C=0xb(x) B=0x0()
raw: K=0x8003(3) S=0x8000(0) L=0x80000000(0) C=0xb(x) B=0x0()
raw: K=0x8004(4) S=0x7fff(-1) L=0x7fffffff(-1) C=0xb(x) B=0x0()
raw: K=0x8005(5) S=0x0() L=0x0() C=0x0() B=0x0()
raw: K=0x8006(6) S=0x0() L=0x0() C=0x0() B=0x0()
raw: K=0x8007(7) S=0x0() L=0x0() C=0x0() B=0x0()
column types: K=0x2502 S=0x1502 L=0x1104 C=0x1d0a B=0x1900
