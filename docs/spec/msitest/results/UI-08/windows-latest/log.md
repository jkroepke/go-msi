# UI-08 / windows-latest

GitHub Actions `windows-latest` runner (interactive desktop, elevated), CI
run 40967817046, 2026-10-29, `TestFullUIRemoveDialog` at commit 3a4cb01
(step "go test (remove page results)").

The package has a `flow: remove` dialog `RemoveOptionsDlg` with a checkbox
on `DELETE_CONFIGURATION` (declared in `properties`) and a component whose
`purge.condition` is `DELETE_CONFIGURATION="1"`. Each removal follows a
quiet install that writes `config.yaml` into the purged folder. The test
passed, so its assertions held:

- keep: it ran `msiexec` with the arguments of the ARP entry's
  `ModifyPath`, whose value the UI-09 log records as
  `MsiExec.exe /I{ProductCode}` (the test at this commit only checked that
  it starts with `MsiExec.exe`), with the box left clear. The product was
  removed, `config.yaml` was still there, and the `Property(S)` lines of
  the run's verbose log had no `DELETE_CONFIGURATION` (the check also
  passes for a missing log; no keep-run excerpt was recorded).
- delete: `msiexec /i` on the package, the box checked. The server's
  properties held `DELETE_CONFIGURATION = 1` and the folder no longer
  existed afterwards.
- `TestICEFullUIRemove` (`requireCleanICE`) found no ICE errors on the
  same package and logged three warnings outside the remove dialog:

  ```text
  ICE03 warning: String overflow (greater than length permitted in column); Table: RadioButton, Column: Text, Key(s): RMOPTION.2
  ICE45 warning: Row 'ErrorDlg.ErrorIcon' in table 'Control' has bits set in the 'Attributes' column that are reserved.
  ICE61 warning: This product should remove only older versions of itself. The Maximum version is not less than the current product. (1.0.0 1.0.0)
  ```

The driver's view of both runs and the excerpt of the delete run's log:

```text
keep:
WINDOW: Next > | Cancel | < Back | DialogBmp | The Setup Wizard lets you change the way MsiBuilderRemoveUI features are installed on your computer or remove it from your computer. Click Next to continue or Cancel to exit the Setup Wizard. | Welcome to the MsiBuilderRemoveUI Setup Wizard
PRESSED: Next >
WINDOW: Repair | Remove | < Back | Cancel | BannerBmp | Select the operation you wish to perform. | Next > | Removes MsiBuilderRemoveUI from your computer. | Repairs errors in the most recent installation by fixing missing and corrupt files, shortcuts and registry entries. | Change, repair, or remove installation
PRESSED: Remove
WINDOW: Delete the configuration | < Back | Next > | Cancel | BannerBmp | What to remove with MsiBuilderRemoveUI. | Remove options
PRESSED: Next >
WINDOW: Remove | Cancel | < Back | BannerBmp | Click Remove to remove MsiBuilderRemoveUI from your computer. Click Back to return to the maintenance menu or Cancel to exit the wizard. | Remove MsiBuilderRemoveUI
PRESSED: Remove
WINDOW: Finish | < Back | DialogBmp | Cancel | Click the Finish button to exit the Setup Wizard. | Completed the MsiBuilderRemoveUI Setup Wizard
PRESSED: Finish
DRIVER: done

delete:
WINDOW: Next > | Cancel | < Back | DialogBmp | The Setup Wizard lets you change the way MsiBuilderRemoveUI features are installed on your computer or remove it from your computer. Click Next to continue or Cancel to exit the Setup Wizard. | Welcome to the MsiBuilderRemoveUI Setup Wizard
PRESSED: Next >
WINDOW: Repair | Remove | < Back | Cancel | BannerBmp | Select the operation you wish to perform. | Next > | Removes MsiBuilderRemoveUI from your computer. | Repairs errors in the most recent installation by fixing missing and corrupt files, shortcuts and registry entries. | Change, repair, or remove installation
PRESSED: Remove
WINDOW: Delete the configuration | < Back | Next > | Cancel | BannerBmp | What to remove with MsiBuilderRemoveUI. | Remove options
CHOSE: Delete the configuration
PRESSED: Next >
WINDOW: Remove | Cancel | < Back | BannerBmp | Click Remove to remove MsiBuilderRemoveUI from your computer. Click Back to return to the maintenance menu or Cancel to exit the wizard. | Remove MsiBuilderRemoveUI
PRESSED: Remove
WINDOW: Finish | < Back | DialogBmp | Cancel | Click the Finish button to exit the Setup Wizard. | Completed the MsiBuilderRemoveUI Setup Wizard
PRESSED: Finish
DRIVER: done

log excerpt:
MSI (c) (34:D8) [22:57:36:956]: PROPERTY CHANGE: Adding DELETE_CONFIGURATION property. Its value is '1'.
MSI (c) (34:E0) [22:57:38:837]: Switching to server: DELETE_CONFIGURATION="1" INSTALLDIR="C:\Program Files\MsiBuilderRemoveUI\" TARGETDIR="D:\" CURRENTDIRECTORY="D:\a\msi-builder\msi-builder\internal\builder\nativetest" CLIENTUILEVEL="0" CLIENTPROCESSID="3892" PRODUCTLANGUAGE="1033" ACTION="INSTALL" EXECUTEACTION="INSTALL" ROOTDRIVE="D:\" INSTALLLEVEL="1" SECONDSEQUENCE="1"  REMOVE=MainFeature
MSI (s) (34:88) [22:57:38:881]: Command Line: DELETE_CONFIGURATION=1 INSTALLDIR=C:\Program Files\MsiBuilderRemoveUI\ TARGETDIR=D:\ CURRENTDIRECTORY=D:\a\msi-builder\msi-builder\internal\builder\nativetest CLIENTUILEVEL=0 CLIENTPROCESSID=3892 PRODUCTLANGUAGE=1033 ACTION=INSTALL EXECUTEACTION=INSTALL ROOTDRIVE=D:\ INSTALLLEVEL=1 SECONDSEQUENCE=1 REMOVE=MainFeature ACTION=INSTALL
MSI (s) (34:88) [22:57:38:881]: PROPERTY CHANGE: Adding DELETE_CONFIGURATION property. Its value is '1'.
MSI (s) (34:88) [22:57:38:981]: Doing action: Purge_Config
Action 22:57:38: Purge_Config. 
Action start 22:57:38: Purge_Config.
MSI (s) (34:88) [22:57:38:982]: MSI_LUA : Custom Action 'Purge_Config' is running with sufficient privileges.
Purge_Config: 
Action ended 22:57:38: Purge_Config. Return value 1.
MSI (s) (34:88) [22:57:39:027]: Executing op: ActionStart(Name=Purge_Config,,)
Action 22:57:39: Purge_Config. 
MSI (s) (34:88) [22:57:39:027]: Executing op: CustomActionSchedule(Action=Purge_Config,ActionType=3170,Source=C:\Windows\system32\,Target="C:\Windows\system32\cmd.exe" /c rmdir /s /q "C:\ProgramData\MsiBuilderRemoveUI\.",)
Property(S): DELETE_CONFIGURATION = 1
Property(S): SecureCustomProperties = DELETE_CONFIGURATION;INSTALLDIR;NEWERVERSIONDETECTED;OLDERVERSIONBEINGUPGRADED
Property(C): DELETE_CONFIGURATION = 1
Property(C): SecureCustomProperties = DELETE_CONFIGURATION;INSTALLDIR;NEWERVERSIONDETECTED;OLDERVERSIONBEINGUPGRADED
```
