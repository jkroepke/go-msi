# UPG-05 / perm-early
built v1.msi sha256 6d838be88f201676ab7108e9a9fac45c6b9d4a3c6d7b0f1d4373e9f4ce0dc6a2
built v2.msi sha256 5a01f798b7b2e4454b164388f9146d432ccea60c29dbc7b1f723697394c411b1
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\perm-early\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bbc.msi
  log: Product: MsiSpec Test upg-perm-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\perm-early\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bc4.msi
  log: Product: MsiSpec Test upg-perm-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT perm-early upgrade: v2 exit 0, v1 registered=false, config="2.0.0-default", registry="2.0.0-default"
msiexec /x {F3FC915E-3128-44D5-97B9-07ED46EDE497} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-perm-early -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT perm-early uninstall: exit 0, v2 registered=false, readme present=false, config="2.0.0-default", registry="2.0.0-default"
  removing leftover component registration {B9B17627-36F7-3E5E-A100-5EBB02F65DF7}
  removing leftover component registration {0DBF4DED-F70D-3DC8-B110-28314DB2B8E9}
