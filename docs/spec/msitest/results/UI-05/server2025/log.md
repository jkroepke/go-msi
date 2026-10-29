# UI-05 / server2025
SERVER2025 (Windows Server 2025, `ver` 10.0.26100.1742), elevated shell in the interactive desktop session (RDP), 2026-10-29.
`go test -v -count=1 -run 'TestFullUIChoice|TestFullUIErrorDialog|TestICEFullUI' ./pkg/builder/nativetest/` at commit b838492.
The package declares `LOG_LEVEL: {}` (no default) and a choice dialog after WelcomeDlg with the options
`{value: warn, label: '&Warnings'}`, `info` and `{value: debug, label: '&Debug'}`; the driver clicks "Debug" with BM_CLICK.
```
WINDOW: Next > | Cancel | < Back | DialogBmp | The Setup Wizard will install MsiBuilderFullUI on your computer. [...]
PRESSED: Next >
WINDOW: Warnings | info | Debug | < Back | Next > | Cancel | BannerBmp | Log level | Log level
CHOSE: Debug
PRESSED: Next >
WINDOW: Browse... | Reset | Disk Usage | < Back | Next > | Cancel | BannerBmp | [...] | Custom Setup
PRESSED: Next >
WINDOW: Install | Cancel | < Back | BannerBmp | [...] | Ready to install MsiBuilderFullUI
PRESSED: Install
WINDOW: Finish | < Back | DialogBmp | Cancel | [...] | Completed the MsiBuilderFullUI Setup Wizard
PRESSED: Finish
DRIVER: done

MSI (c) [12:49:24:270]: PROPERTY CHANGE: Modifying LOG_LEVEL property. Its current value is 'warn'. Its new value: 'debug'.
MSI (c) [12:49:27:757]: Switching to server: APPLICATIONFOLDER="C:\Program Files\MsiBuilderFullUI\" LOG_LEVEL="debug" [...]
Property(S): LOG_LEVEL = debug
Property(S): SecureCustomProperties = APPLICATIONFOLDER;LOG_LEVEL;NEWERVERSIONDETECTED;OLDERVERSIONBEINGUPGRADED
--- PASS: TestFullUIChoice (9.27s)
--- PASS: TestFullUIErrorDialog (10.87s)
TestICEFullUI: no errors; warnings ICE03 (RadioButton.Text RMOPTION/2, the files-in-use dialog), ICE45 (ErrorDlg.ErrorIcon), ICE61
--- PASS: TestICEFullUI (1.69s)
```
The product was uninstalled afterwards and `C:\Program Files\MsiBuilderFullUI` was gone.
