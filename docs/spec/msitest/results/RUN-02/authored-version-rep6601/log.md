# RUN-02 / authored-version-rep6601
built v1.msi sha256 bcf67ca1b5297e4aa6c2f1d9169025538af6641a193d17e08b64b5178fb6a452
  set File.Version 1.0.0.0: <nil>, commit <nil>
built v2.msi sha256 9c5039c1cdb7db78cbfb9b4eb57ee2709529042eb037406bb7461f4a7a390186
  set File.Version 2.0.0.0: <nil>, commit <nil>
msiexec /i results\RUN-02\authored-version-rep6601\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a64d.msi
  log: Product: MsiSpec Test run02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-02\authored-version-rep6601\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\$PatchCache$\Managed\A65566646EFA084429ECCDBB63408E73\1.0.0\f_app
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a652.msi
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\$PatchCache$\Managed\A65566646EFA084429ECCDBB63408E73\CacheSize.txt
  log: Product: MsiSpec Test run02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT authored-version-rep6601: upgrade exit 0, installed app.exe = v2 bytes, v1 registered=false
msiexec /fa results\RUN-02\authored-version-rep6601\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test run02 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
  repair /fa after deleting file: exit 0, file restored=true
msiexec /x {72D9350C-52F6-489E-A7A3-E716BEED27DC} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run02 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {72D9350C-52F6-489E-A7A3-E716BEED27DC} -> 0
