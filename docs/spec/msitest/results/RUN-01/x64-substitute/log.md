# RUN-01 / x64-substitute
Published test-vectors/minimal.msi is not in the repository; this is a labelled substitute built with the native importer.
built control.msi sha256 51fd4642f30045fd1c051af3644c05e0a370ced850cbb464eb0543fe029bad29
msiexec /i results\RUN-01\x64-substitute\control.msi -> exit 0
  log: Note: 1: 2203 2: C:\WINDOWS\Installer\inprogressinstallinfo.ipi 3: -2147287038
  log: Note: 1: 2262 2: DigitalSignature 3: -2147287038
  log: Note: 1: 2205 2:  3: MsiFileHash
  log: Note: 1: 2205 2:  3: Control
  log: Note: 1: 2205 2:  3: Error
  log: Note: 1: 2262 2: AdminProperties 3: -2147287038
  log: Note: 1: 2898 2: MS Sans Serif 3: MS Sans Serif 4: 0 5: 16
  log: Note: 1: 2205 2:  3: PatchPackage
  log: Note: 1: 2205 2:  3: LaunchCondition
  log: Note: 1: 2228 2:  3: LaunchCondition 4: SELECT `Condition` FROM `LaunchCondition`
  log: Note: 1: 2205 2:  3: ActionText
  log: Note: 1: 2205 2:  3: Upgrade
  ... 49 more log hits
  installed file C:\Program Files\MsiSpecTest\readme.txt: ok sha ae87e3113c625bff29f58c7aba980be846743f26c35d207685d15da6da4ade12
msiexec /x results\RUN-01\x64-substitute\control.msi -> exit 0
  log: Note: 1: 2203 2: C:\WINDOWS\Installer\inprogressinstallinfo.ipi 3: -2147287038
  log: Note: 1: 2205 2:  3: MsiFileHash
  log: Note: 1: 2205 2:  3: Control
  log: Note: 1: 2205 2:  3: Error
  log: Note: 1: 2262 2: AdminProperties 3: -2147287038
  log: Note: 1: 2898 2: MS Sans Serif 3: MS Sans Serif 4: 0 5: 16
  log: Note: 1: 2205 2:  3: PatchPackage
  log: Note: 1: 2205 2:  3: LaunchCondition
  log: Note: 1: 2228 2:  3: LaunchCondition 4: SELECT `Condition` FROM `LaunchCondition`
  log: Note: 1: 2205 2:  3: ActionText
  log: Note: 1: 2228 2:  3: LaunchCondition 4: SELECT `Condition`, `Description` FROM `LaunchCondition`
  log: Note: 1: 2205 2:  3: Patch
  ... 43 more log hits
RESULT install+uninstall ok=true target=C:\Program Files\MsiSpecTest\readme.txt
