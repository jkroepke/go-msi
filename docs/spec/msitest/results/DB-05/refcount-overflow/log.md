# DB-05 / refcount-overflow
n=32766: pool entries holding 'same': [id 8 len 4 ref 32766]; total pool 8
n=32766: stored refcount of 'same' (id 8) = 32766 (real refs 32766); longRefs false
  native rows with V='same': 32766 err=<nil>
  native K=1 -> [["same"]] err=<nil>
  raw first V id: 8 (stream 196596 bytes)
  raw _StringPool tail: 01 00 01 00 01 00 01 00 04 00 fe 7f
n=32767: pool entries holding 'same': [id 8 len 4 ref 32767]; total pool 8
n=32767: stored refcount of 'same' (id 8) = 32767 (real refs 32767); longRefs false
  native rows with V='same': 32767 err=<nil>
  native K=1 -> [["same"]] err=<nil>
  raw first V id: 8 (stream 196602 bytes)
  raw _StringPool tail: 01 00 01 00 01 00 01 00 04 00 ff 7f
n=32768: pool entries holding 'same': []; total pool 7
n=32768: stored refcount of 'same' (id 0) = 0 (real refs 32768); longRefs false
  native rows with V='same': 32768 err=<nil>
  native K=1 -> [["\x00NULL"]] err=<nil>
  raw first V id: 8 (stream 196608 bytes)
  raw _StringPool tail: 04 00 03 00 01 00 01 00 01 00 01 00
n=65534: pool entries holding 'same': [id 8 len 4 ref 32766]; total pool 8
n=65534: stored refcount of 'same' (id 8) = 32766 (real refs 65534); longRefs false
  native rows with V='same': 65534 err=<nil>
  native K=1 -> [["same"]] err=<nil>
  raw first V id: 8 (stream 393204 bytes)
  raw _StringPool tail: 01 00 01 00 01 00 01 00 04 00 fe 7f
n=65535: pool entries holding 'same': [id 8 len 4 ref 32767]; total pool 8
n=65535: stored refcount of 'same' (id 8) = 32767 (real refs 65535); longRefs false
  native rows with V='same': 65535 err=<nil>
  native K=1 -> [["same"]] err=<nil>
  raw first V id: 8 (stream 393210 bytes)
  raw _StringPool tail: 01 00 01 00 01 00 01 00 04 00 ff 7f
n=65536: pool entries holding 'same': []; total pool 7
n=65536: stored refcount of 'same' (id 0) = 0 (real refs 65536); longRefs false
  native rows with V='same': 65536 err=<nil>
  native K=1 -> [["\x00NULL"]] err=<nil>
  raw first V id: 8 (stream 393216 bytes)
  raw _StringPool tail: 04 00 03 00 01 00 01 00 01 00 01 00
n=70000: pool entries holding 'same': [id 8 len 4 ref 4464]; total pool 8
n=70000: stored refcount of 'same' (id 8) = 4464 (real refs 70000); longRefs false
  native rows with V='same': 70000 err=<nil>
  native K=1 -> [["same"]] err=<nil>
  raw first V id: 8 (stream 420000 bytes)
  raw _StringPool tail: 01 00 01 00 01 00 01 00 04 00 70 11
