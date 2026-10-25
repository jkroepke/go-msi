# RUN-17 / rep-6551
built v1.msi sha256 7e323bf13f8799fa21856d6418b2640e3cd0de96bb1b27c2624184584375e83e
built v2.msi sha256 d57a857898371d36397e5364c3a6b2abd7a3392c5af3032cb369670adc842533
msiexec /i results\RUN-17\rep-6551\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5e5.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-17\rep-6551\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5ea.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT REP at 6551: v2 exit 0, v1 registered=false, v2 registered=true, file present=true
msiexec /x {B4BD9A34-B2FA-43E0-972D-0D559046BD17} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {B4BD9A34-B2FA-43E0-972D-0D559046BD17} -> 0
