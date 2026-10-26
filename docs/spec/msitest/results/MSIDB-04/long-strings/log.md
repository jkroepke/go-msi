# MSIDB-04 / long-strings
insert 65534 bytes: MsiViewFetch: 1627
insert 65535 bytes: MsiViewFetch: 1627
insert 65536 bytes: MsiViewFetch: 1627
insert 70000 bytes: MsiViewFetch: 1627
commit: <nil>
native read k65534 -> 65534 chars
native read k65535 -> 65535 chars
native read k65536 -> 65536 chars
native read k70000 -> 70000 chars
native read k_small -> 5 chars
pool id 8: len 65534 long=false raw words [65534 1 6 1]
pool id 10: len 65535 long=false raw words [65535 1 6 1]
pool id 12: len 65536 long=true raw words [0 1 0 1]
pool id 14: len 70000 long=true raw words [0 1 4464 1]
raw reader with layout (0, refcount, low, high) decoded pool OK; _StringData 266647 bytes
