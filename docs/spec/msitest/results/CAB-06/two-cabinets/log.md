# CAB-06 / two-cabinets
built correct.msi sha256 19f155545a0fc2cbf7a793aa7adc9e413bce2da9d17171f0d519664d3161e9c0
insert cab1: <nil>
insert cab2: <nil>
commit <nil>
msiexec /i results\CAB-06\two-cabinets\correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7db.msi
  log: Product: MsiSpec Test cab06 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT correct: install 0, a.txt ok=true, b.txt ok=true
msiexec /x results\CAB-06\two-cabinets\correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab06 -- Removal completed successfully.
  log: MainEngineThread is returning 0
built lastsequence-too-high.msi sha256 589e98fb53cf4a785717401e0b94547d329fb3ad2b27cc611c7b67116df8d020
insert cab1: <nil>
insert cab2: <nil>
commit <nil>
msiexec /i results\CAB-06\two-cabinets\lastsequence-too-high.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1334. The file 'f_b' cannot be installed because the file cannot be found in cabinet file 'cab1.cab'. This could indicate a network error, an error reading from the
  log: Return value 3.
  log: Product: MsiSpec Test cab06 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT lastsequence-too-high: install 1603, a.txt ok=false, b.txt ok=false
built lastsequence-decreasing.msi sha256 9da23b00b931ed934c886eca97cad06c1b93ec2b2435943fac2172675d6a15de
insert cab1: <nil>
insert cab2: <nil>
commit <nil>
msiexec /i results\CAB-06\two-cabinets\lastsequence-decreasing.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1334. The file 'f_b' cannot be installed because the file cannot be found in cabinet file 'cab1.cab'. This could indicate a network error, an error reading from the
  log: Return value 3.
  log: Product: MsiSpec Test cab06 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT lastsequence-decreasing: install 1603, a.txt ok=false, b.txt ok=false
built cabinet-name-case.msi sha256 247dca5b11da5d6fa13822af5a1d18a5461290534e37a55c18b84d2489f7c428
insert cab1: <nil>
insert cab2: <nil>
commit <nil>
msiexec /i results\CAB-06\two-cabinets\cabinet-name-case.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2356 2: CAB1.cab
  log: Error 2356:  Couldn't locate cabinet in stream: CAB1.cab.
  log: Return value 3.
  log: Product: MsiSpec Test cab06 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT cabinet-name-case: install 1603, a.txt ok=false, b.txt ok=false
