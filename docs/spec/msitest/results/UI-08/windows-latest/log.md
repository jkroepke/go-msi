# UI-08 / windows-latest

GitHub Actions `windows-latest` runner (interactive desktop, elevated), CI
run 40967817046, 2026-10-29, `TestFullUIRemoveDialog` at commit 3a4cb01
(step "go test (remove page results)").

The package has a `flow: remove` dialog `RemoveOptionsDlg` with a checkbox
on `DELETE_CONFIGURATION` (declared in `properties`) and a component whose
`purge.condition` is `DELETE_CONFIGURATION="1"`. The first removal runs the
ARP entry's `ModifyPath` command with the box left clear: the folder and its
`config.yaml` stay, and the server has no `DELETE_CONFIGURATION`. The last
one runs `msiexec /i` on the package and checks the box: the server gets the
value and `Purge_Config` deletes the folder. `TestICEFullUIRemove` ran the
ICE suite on the same package without findings.

```text
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
MSI (c) (34:E0) [22:57:38:837]: Switching to server: DELETE_CONFIGURATION="1" INSTALLDIR="C:\Program Files\MsiBuilderRemoveUI\" TARGETDIR="D:\" CURRENTDIRECTORY="D:\a\msi-builder\msi-builder\internal\builder
ativetest" CLIENTUILEVEL="0" CLIENTPROCESSID="3892" PRODUCTLANGUAGE="1033" ACTION="INSTALL" EXECUTEACTION="INSTALL" ROOTDRIVE="D:\" INSTALLLEVEL="1" SECONDSEQUENCE="1"  REMOVE=MainFeature
MSI (s) (34:88) [22:57:38:881]: Command Line: DELETE_CONFIGURATION=1 INSTALLDIR=C:\Program Files\MsiBuilderRemoveUI\ TARGETDIR=D:\ CURRENTDIRECTORY=D:\a\msi-builder\msi-builder\internal\builder
ativetest CLIENTUILEVEL=0 CLIENTPROCESSID=3892 PRODUCTLANGUAGE=1033 ACTION=INSTALL EXECUTEACTION=INSTALL ROOTDRIVE=D:\ INSTALLLEVEL=1 SECONDSEQUENCE=1 REMOVE=MainFeature ACTION=INSTALL
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
