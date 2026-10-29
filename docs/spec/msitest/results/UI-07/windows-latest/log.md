# UI-07 / windows-latest

GitHub Actions `windows-latest` runner (interactive desktop, elevated), CI
run 40882656575, 2026-10-29, `TestFullUIFinishText` at commit 2869138.
The package sets
`finishText: Open a new command prompt to use [ProductName] & its tools.`
The driver installs with the full UI, then runs `msiexec /i` again and
presses Next, Repair (maintenance menu), Repair (VerifyRepairDlg) and Finish.
The test failed at that commit only because it expected the `&` in the UI
Automation name; the names drop it, as they drop the access-key `&` of every
button. Commit 5b2e3fd compares without it. The driver at that commit also
logged every off-screen element with a name as a `HIDDEN:` line and logged
none, so the hidden control is absent from the control view rather than reported
off-screen; later commits dropped that logging.

```text
install:
WINDOW: Next > | Cancel | < Back | DialogBmp | The Setup Wizard will install MsiBuilderFullUI on your computer. [...] | Welcome to the MsiBuilderFullUI Setup Wizard
PRESSED: Next >
WINDOW: Warnings | info | Debug | < Back | Next > | Cancel | BannerBmp | Log level | Log level
PRESSED: Next >
WINDOW: Browse... | Reset | Disk Usage | < Back | Next > | Cancel | BannerBmp | [...] | Custom Setup
PRESSED: Next >
WINDOW: Install | Cancel | < Back | BannerBmp | [...] | Ready to install MsiBuilderFullUI
PRESSED: Install
WINDOW: Finish | < Back | DialogBmp | Cancel | Click the Finish button to exit the Setup Wizard. | Open a new command prompt to use MsiBuilderFullUI  its tools. | Completed the MsiBuilderFullUI Setup Wizard
PRESSED: Finish
DRIVER: done

repair:
WINDOW: Next > | Cancel | < Back | DialogBmp | The Setup Wizard lets you change the way MsiBuilderFullUI features are installed [...] | Welcome to the MsiBuilderFullUI Setup Wizard
PRESSED: Next >
WINDOW: Change | Repair | Remove | < Back | Cancel | BannerBmp | [...] | Next > | [...] | Change, repair, or remove installation
PRESSED: Repair
WINDOW: Repair | Cancel | < Back | BannerBmp | [...] | Ready to repair MsiBuilderFullUI
PRESSED: Repair
WINDOW: Finish | < Back | DialogBmp | Cancel | Click the Finish button to exit the Setup Wizard. | Completed the MsiBuilderFullUI Setup Wizard
PRESSED: Finish
DRIVER: done
```

After the install the finish page lists the paragraph with `[ProductName]`
resolved. After the repair the `FinishText` control, hidden by
`Installed OR ACTION = "ADMIN"`, is not in the UI Automation control view that the driver walks
(`FindAll` on descendants without a cache request).
The product was uninstalled by the test's cleanup.
