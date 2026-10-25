# VAL-01 / msival2-compare

Host: Windows Server 2025 (SERVER2025), 2026-10-25.

Inputs:

- winsdksetup.exe 10.1.28000.2957 (SDK 10.0.28000.2957) from
  `https://go.microsoft.com/fwlink/?linkid=2382519`, SHA-256
  `E8E264ACA8BC34ABDF592352BF09F986A85CDB96DB0EFBA405CA9E149181FE61`,
  Microsoft Authenticode signature.
- `winsdksetup.exe /features OptionId.MSIInstallTools /layout C:\sdk /quiet /norestart`
  works unattended (about 55 MB). A long or 8.3 layout path fails with exit
  code 2002 "Invalid download path".
- `Installers\MsiVal2-x86_en-us.msi`, SHA-256
  `74BCFD1B9DEB0AF32EC8A1CE95D7340DC2F6FE0510D534A92AEE6E7F111D9C26`,
  installs with `msiexec /i ... /qn` (exit 0).
- `C:\Program Files (x86)\MsiVal2\darice.cub`, 692224 bytes, SHA-256
  `9C5E0525F68E8522860D106FE4C7BD77B1742F70E37EA0962687B1C1D5E2293B`.
- msival2.exe 5.0.28000.2957, SHA-256
  `2040DDF023A7F2B7B7C62ACF6BBF022BFB942CECD083A109CC824D27A7262001`.

msival2.exe as installed prints `Failed to instantiate EvalCom Object`: the
package registers CLSID `{DC550E10-DBA5-11D1-A850-006097ABDE17}`, while
msival2.exe and evalcom2.dll use `{6E5E1910-8053-4660-B795-6B612E29BC58}`.
Registering that CLSID under `HKLM\SOFTWARE\Classes\WOW6432Node\CLSID`
(InprocServer32 evalcom2.dll, ThreadingModel Apartment) makes it work.

CI (GitHub `windows-latest`, Windows Server 2025, 2026-10-25) installs
winget package `Microsoft.WindowsSDK.10.0.28000` version 10.0.28000.2526
(SDK 10.0.28000.1721) with `--override '/features OptionId.MSIInstallTools
/quiet /norestart'`. Its `darice.cub` differs from the one above. Two runs with the same pinned
manifest installed cubs with different SHA-256 values,
`0EB158691FCEE38DDA2E342A94E39164B9AE7943A286C031449F6D257875EC1B` and
`8B0285750835A42EF47AA0E27B8E93B3877F7F6C26BB6B28F4E5D23A68C16BC8` (observed,
cause unknown), so CI logs the cub's hash and does not enforce one. The
msival2.exe comparison below used the 10.0.28000.2957 cub only; with the CI
cub, the examples pass the same Error/Failure gate in `TestICEExamples` and
ICE08 still reports the duplicate GUID (observed in CI).

Go runner (`internal/ice`), first attempt:

- `MsiOpenPackageExW` with `MSIOPENPACKAGEFLAGS_IGNOREMACHINESTATE`: every
  ICE action returns 1626; the log says
  `Action 'ICE01' is not permitted in a restricted engine`. Flags 0 work.
- `MsiDatabaseMergeW` returns 1627 for every package: 1 conflict in
  `Property` (the cub's placeholder `ProductCode`) and 3 in `_Validation`.
  The package's rows are kept. ICE03 then reports the `_MergeErrors` table
  itself, so it is dropped before the ICEs run.
- The VBScript ICEs (ICE08, ICE61) run.

Comparison, `msival2.exe <msi> darice.cub -f` against the Go runner (ICE,
type, description; table, column and keys where msival2 prints them, which
is ICE03 only). `-f` leaves out Info lines (type 3), so the Go counts
leave them out too; the runner itself returns them:

| Package | msival2 | Go | Differences |
|---|---|---|---|
| minimal | 1 | 1 | none |
| windows_exporter | 25 | 25 | none |
| duplicate ComponentId | 26 | 26 | none |

Both report `ICE08 ERROR Component: TextfileDirectory has a duplicate GUID:
{5546455E-B5EA-5AF8-A21A-AA526F04D083}`; the Go runner adds
`Component.ComponentId` key `TextfileDirectory`.

Findings on the examples (warnings only, no errors):

- minimal: ICE61 (Upgrade maximum version 1.0.0 equals the current version,
  Attributes 513).
- windows_exporter: ICE61 (same, 0.32.0.5); ICE03 string overflow in
  `RadioButton.Text` RMOPTION/2 and `CustomAction.Target` KillProcess;
  ICE45 x22, reserved bits in `Control.Attributes` of the Title and
  Description controls of BrowseDlg, ConfigDlg, CustomizeDlg, DiskCostDlg,
  FilesInUse, MaintenanceTypeDlg, MsiRMFilesInUse, ProgressDlg,
  VerifyReadyDlg, VerifyRemoveDlg and VerifyRepairDlg.

RESULT the msi.dll runner matches msival2.exe; Darice.cub can be used from Go
without MsiVal2 or COM.
