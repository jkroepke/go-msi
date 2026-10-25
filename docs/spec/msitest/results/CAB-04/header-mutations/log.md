# CAB-04 / header-mutations
built control.msi sha256 15dcdbb6aef6ed94b9200a542b56b98c4f54469ded76d348d53f698c3945be08
msiexec /i results\CAB-04\header-mutations\control.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a789.msi
  log: Product: MsiSpec Test cab04 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CAB-04\header-mutations\control.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab04 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT control: expand.exe "Expanding Files Complete ..." extracted-equal=false; msiexec 0 installed-equal=true
built minor-2.msi sha256 9cc545db7f1eadaedb0580e31f2d5811b1e3b50b926732702bc93c79de8238ce
msiexec /i results\CAB-04\header-mutations\minor-2.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2356 2: cab1.cab
  log: Error 2356:  Couldn't locate cabinet in stream: cab1.cab.
  log: Return value 3.
  log: Product: MsiSpec Test cab04 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT minor-2: expand.exe "results\\cab-04\\header-mutations\\minor-2.cab: 1715 bytes copied." extracted-equal=false; msiexec 1603 installed-equal=false
built major-2.msi sha256 3eadf015629e141f42632b788aa2e20fdf133155a4911334d5d68d130ce09963
msiexec /i results\CAB-04\header-mutations\major-2.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2356 2: cab1.cab
  log: Error 2356:  Couldn't locate cabinet in stream: cab1.cab.
  log: Return value 3.
  log: Product: MsiSpec Test cab04 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT major-2: expand.exe "results\\cab-04\\header-mutations\\major-2.cab: 1715 bytes copied." extracted-equal=false; msiexec 1603 installed-equal=false
built cbCabinet-plus-100.msi sha256 e89225385757e636c92d691525a42fe986dd9cfb00e754db2022352f0cf56ed6
msiexec /i results\CAB-04\header-mutations\cbCabinet-plus-100.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a796.msi
  log: Product: MsiSpec Test cab04 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CAB-04\header-mutations\cbCabinet-plus-100.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab04 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT cbCabinet-plus-100: expand.exe "Expanding Files Complete ..." extracted-equal=false; msiexec 0 installed-equal=true
built cbCabinet-minus-100.msi sha256 4f2b16e0b207d5dcea499b5eef69ab5a8dfcb2b71e7467dcf94e562a399d84ea
msiexec /i results\CAB-04\header-mutations\cbCabinet-minus-100.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a79d.msi
  log: Product: MsiSpec Test cab04 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CAB-04\header-mutations\cbCabinet-minus-100.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab04 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT cbCabinet-minus-100: expand.exe "Expanding Files Complete ..." extracted-equal=false; msiexec 0 installed-equal=true
built checksum-wrong.msi sha256 3fe248bfbba86248500707f5ba41f9a4bea9e1a6355b388f94f615497bb08d2f
msiexec /i results\CAB-04\header-mutations\checksum-wrong.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1335. The cabinet file 'cab1.cab' required for this installation is corrupt and cannot be used. This could indicate a network error, an error reading from the CD-RO
  log: Return value 3.
  log: Product: MsiSpec Test cab04 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT checksum-wrong: expand.exe "Expanding File results\\cab-04\\header-mutations\\checksum-wrong.cab Incomplete, Error Code=0xca00a009" extracted-equal=false; msiexec 1603 installed-equal=false
built checksum-zero.msi sha256 d0ad32bfce636218e0db5218b5066a9c3c64797072c481b90889def34c8ec5a9
msiexec /i results\CAB-04\header-mutations\checksum-zero.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7a7.msi
  log: Product: MsiSpec Test cab04 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CAB-04\header-mutations\checksum-zero.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab04 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT checksum-zero: expand.exe "Expanding Files Complete ..." extracted-equal=false; msiexec 0 installed-equal=true
built payload-bitflip.msi sha256 68afcb7643dfbc1a1a507ccbdf7183fa88b4de13473ab026ce3870826992f2f8
msiexec /i results\CAB-04\header-mutations\payload-bitflip.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1335. The cabinet file 'cab1.cab' required for this installation is corrupt and cannot be used. This could indicate a network error, an error reading from the CD-RO
  log: Return value 3.
  log: Product: MsiSpec Test cab04 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT payload-bitflip: expand.exe "Expanding File results\\cab-04\\header-mutations\\payload-bitflip.cab Incomplete, Error Code=0xca00a009" extracted-equal=false; msiexec 1603 installed-equal=false
built payload-bitflip-checksum-zero.msi sha256 1016c399e809beb2c21156db448bc816869fb8549e50495251d5eea5965688a2
msiexec /i results\CAB-04\header-mutations\payload-bitflip-checksum-zero.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7b1.msi
  log: Product: MsiSpec Test cab04 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CAB-04\header-mutations\payload-bitflip-checksum-zero.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab04 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT payload-bitflip-checksum-zero: expand.exe "Expanding Files Complete ..." extracted-equal=false; msiexec 0 installed-equal=false
