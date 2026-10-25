# DB-05 / refcount-read
variant ref-1.msi sha256 984b83564a460d45b9228c640b692643c294e39c1239651d97ec26a2b60135ec
RESULT stored refcount 1: native read K=1 -> [["same"]] err=<nil>; after native insert+commit (<nil>/<nil>) refcount -> 1
variant ref-32767.msi sha256 43d7613cf818a18f23f04662f562cea9f0a6d7c3c71fc27aa88a47c81d5428cb
RESULT stored refcount 32767: native read K=1 -> [["same"]] err=<nil>; after native insert+commit (<nil>/<nil>) refcount -> 32767
variant ref-32768.msi sha256 2529f8e26aa6e6d8c743ba9b50cc875fc58cc389f0e35d89c32f7948aacc9968
RESULT stored refcount 32768: native read K=1 -> [["same"]] err=<nil>; after native insert+commit (<nil>/<nil>) refcount -> missing
variant ref-32769.msi sha256 172ceaf4602866f640941c8e3b4a365e826a6339cdf26520da74e8f44fd87bc3
RESULT stored refcount 32769: native read K=1 -> [["same"]] err=<nil>; after native insert+commit (<nil>/<nil>) refcount -> 1
variant ref-65535.msi sha256 8fc0bad1ca9bb59266516eaa3ff4dac118525599ddc47a4326b3d440e579128d
RESULT stored refcount 65535: native read K=1 -> [["same"]] err=<nil>; after native insert+commit (<nil>/<nil>) refcount -> 32767
variant ref-0.msi sha256 f9a3985bb763f0b772d92443a55df6c7689e30228182d47a519c500dc5c07eef
RESULT stored refcount 0: native read K=1 -> [["same"]] err=<nil>; after native insert+commit (<nil>/<nil>) refcount -> missing
