# UPG-04 / rep-6601
built v1.msi sha256 05375188a5ba6b184ac82262601430ea34449db29e414a905a4e2ef5179b7a49
built v2.msi sha256 7b4107bff3c63111fb709caaf8ea640ab66e5375525bad57f6fc67b6cdd7393b
msiexec /i results\UPG-04\rep-6601\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5f3.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\UPG-04\rep-6601\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5f8.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT REP at 6601: v2 exit 0, v1 registered=false, v2 registered=true, file present=true
msiexec /x {9E51A86C-D3F4-4427-8F7E-6D2C0794EC47} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {9E51A86C-D3F4-4427-8F7E-6D2C0794EC47} -> 0
