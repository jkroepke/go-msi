# SEARCH-05 / probe
GitHub Actions `windows-latest` (Windows Server 2025, 10.0.26100), elevated, time zone UTC, 2026-10-29.
Temporary test `TestProbeSearchLocators` (`internal/builder/nativetest/probe_search_windows_test.go`, commit 86cd407
on pull request #111, run 40904980113; removed before merging). It built a package with msi-builder, added raw
`AppSearch`, `Signature`, `DrLocator`, `CompLocator` and `RegLocator` rows, installed it with `msiexec /i /qn /l*v`
and read the `Property(S):` lines. Exit code 0.

Files: `C:\MsiBuilderProbe\sub\app.exe` (version 1.2.3.4, language 1033, 2048 bytes, written during the test),
`neutral.exe` (1.0.0.0, language 0), `nolang.exe` (1.0.0.0, no translation), `plain.txt` (no version),
`long file name here.txt`, `deep\a\b\deep.txt`. Registry (64-bit view) `HKLM\SOFTWARE\MsiBuilderProbe`:
`Dir` = `C:\MsiBuilderProbe`, `DirSlash` = `C:\MsiBuilderProbe\`, `File` = `C:\MsiBuilderProbe\sub\app.exe`.
A second package installed before it holds component `{6A0D3C2E-5B1F-4E7A-9C3D-2F8E1B4A7C60}` with key file
`C:\Program Files\MsiBuilderProbeBase\base.exe`. Rows are `DrLocator(Parent, Path, Depth)`,
`Signature(FileName, MinVersion, MaxVersion, MinSize, MaxSize, MinDate, MaxDate, Languages)`,
`CompLocator(ComponentId, Type)`, `RegLocator(Name, Type)`; a signature without a `Signature` row is a folder search.
`S_REGDIR_PARENT` is `RegLocator(Dir, 0|16)` and `S_DIR_PARENT` is `DrLocator(-, C:\MsiBuilderProbe, -)`; neither
has an `AppSearch` row of its own.

```
Folders (no Signature row)
P_DIR_ABS              DrLocator(-, C:\MsiBuilderProbe, -)              = "C:\MsiBuilderProbe\"
P_DIR_SLASH            DrLocator(-, C:\MsiBuilderProbe\, -)             = "C:\MsiBuilderProbe\"
P_DIR_PROP             DrLocator(-, [WindowsFolder]System32, 0)         = "C:\Windows\System32\"
P_DIR_MISSING          DrLocator(-, C:\MsiBuilderNoSuchFolder, -)       = <unset>
P_DIR_FILE             DrLocator(-, C:\MsiBuilderProbe\plain.txt, -)    = <unset>
P_DIR_UNDER_REG        DrLocator(S_REGDIR_PARENT, sub, -)               = "C:\MsiBuilderProbe\sub\"
P_DIR_UNDER_REG_MISS   DrLocator(S_REGDIR_PARENT, nosuch, -)            = <unset>
P_DIR_UNDER_DIR_DEPTH0 DrLocator(S_DIR_PARENT, b, 0)                    = <unset>
P_DIR_UNDER_DIR_DEPTH2 DrLocator(S_DIR_PARENT, b, 2)                    = "C:\MsiBuilderProbe\deep\a\b\"
P_DIR_UNDER_DIR_MULTI  DrLocator(S_DIR_PARENT, deep\a, 0)               = "C:\MsiBuilderProbe\deep\a\"
P_DIR_ABS_DEPTH        DrLocator(-, C:\MsiBuilderProbe\b, 3)            = "C:\MsiBuilderProbe\deep\a\b\"

Files (Signature + DrLocator)
P_FILE_ABS             app.exe in C:\MsiBuilderProbe\sub, 0             = "C:\MsiBuilderProbe\sub\app.exe"
P_FILE_NODEPTH         app.exe in ...\sub, Depth null                   = "C:\MsiBuilderProbe\sub\app.exe"
P_FILE_CASE            APP.EXE in ...\sub, 0                            = "C:\MsiBuilderProbe\sub\APP.EXE"
P_FILE_DEPTH0_MISS     app.exe in C:\MsiBuilderProbe, 0                 = <unset>
P_FILE_DEPTH1          app.exe in C:\MsiBuilderProbe, 1                 = "C:\MsiBuilderProbe\sub\app.exe"
P_FILE_DEEP2           deep.txt in C:\MsiBuilderProbe, 2                = <unset>
P_FILE_DEEP3           deep.txt in C:\MsiBuilderProbe, 3                = "C:\MsiBuilderProbe\deep\a\b\deep.txt"
P_FILE_LONG            "long file name here.txt" in C:\MsiBuilderProbe  = "C:\MsiBuilderProbe\long file name here.txt"
P_FILE_PATH_FILE       app.exe in C:\MsiBuilderProbe\sub\app.exe, 0     = <unset>
P_FILE_MISSING         nosuch.exe in ...\sub                            = <unset>
P_FILE_UNDER_REG       app.exe, DrLocator(S_REGDIR_PARENT, sub, 0)      = "C:\MsiBuilderProbe\sub\app.exe"
P_FILE_UNDER_REG_DEPTH app.exe, DrLocator(S_REGDIR_PARENT, -, 1)        = "C:\MsiBuilderProbe\sub\app.exe"
P_FILE_UNDER_DIR       deep.txt, DrLocator(S_DIR_PARENT, deep\a\b, 0)   = "C:\MsiBuilderProbe\deep\a\b\deep.txt"

Bounds on app.exe (1.2.3.4, language 1033, 2048 bytes)
P_VER_MIN_LOW          MinVersion 1.0.0.0                               = found
P_VER_MIN_SHORT        MinVersion 1.2                                   = found
P_VER_MIN_EQ_NOLANG    MinVersion 1.2.3.4, Languages null               = <unset>
P_VER_MIN_EQ_1033      MinVersion 1.2.3.4, Languages 1033               = found
P_VER_MIN_EQ_1031      MinVersion 1.2.3.4, Languages 1031               = <unset>
P_VER_MIN_LOW_1031     MinVersion 1.0.0.0, Languages 1031               = found
P_VER_MIN_HIGH         MinVersion 1.2.3.5                               = <unset>
P_VER_MAX_LOW          MaxVersion 1.2.3.3                               = <unset>
P_VER_MAX_EQ           MaxVersion 1.2.3.4                               = found
P_VER_MAX_HIGH         MaxVersion 2.0.0.0                               = found
P_VER_UNVERSIONED      plain.txt, MinVersion 0.0.0.1                    = <unset>
P_VER_NEUTRAL_EQ       neutral.exe (1.0.0.0, language 0), MinVersion 1.0.0.0, Languages null = <unset>
P_VER_NOLANG_EQ        nolang.exe (1.0.0.0, none), MinVersion 1.0.0.0, Languages null       = "C:\MsiBuilderProbe\nolang.exe"
P_SIZE_MIN_EQ          MinSize 2048                                     = found
P_SIZE_MIN_OVER        MinSize 2049                                     = <unset>
P_SIZE_MAX_EQ          MaxSize 2048                                     = found
P_SIZE_MAX_UNDER       MaxSize 2047                                     = <unset>
P_DATE_MIN_PAST        MinDate 2000-01-01 00:00                         = found
P_DATE_MIN_FUTURE      MinDate 2040-01-01 00:00                         = <unset>
P_DATE_MAX_PAST        MaxDate 2000-01-01 00:00                         = <unset>
P_DATE_MAX_FUTURE      MaxDate 2040-01-01 00:00                         = found
P_DATE_MIN_UTC_MINUS1H MinDate modification time - 1 h                 = found
P_DATE_MIN_UTC_PLUS1H  MinDate modification time + 1 h                 = <unset>
P_DATE_MIN_LOCAL_MINUS1H (same as UTC on this runner)                   = found
kernel32.dll 10.0.26100.33648 (language 1033), [System64Folder], depth 0:
P_OS_MIN_REAL_MAJOR    MinVersion 10.0.0.0                              = <unset>
P_OS_MIN_REAL_BUILD    MinVersion 10.0.26100.0                          = <unset>
P_OS_MAX_63            MaxVersion 6.3.0.0                               = <unset>

RegLocator with Type 1 (the value names a file) and Type 0
P_REG_FILE             File, 1|16, Signature app.exe                    = "C:\MsiBuilderProbe\sub\app.exe"
P_REG_FILE_WRONGNAME   File, 1|16, Signature other.exe                  = "C:\MsiBuilderProbe\sub\app.exe"
P_REG_FILE_MINVER      File, 1|16, Signature app.exe MinVersion 1.2.3.5 = <unset>
P_REG_FILE_NOSIG       File, 1|16, no Signature row                     = "C:\MsiBuilderProbe\sub\"
P_REG_DIRSIG           Dir, 0|16, Signature plain.txt                   = "C:\MsiBuilderProbe\plain.txt"
P_REG_DIRSLASH         DirSlash, 0|16                                   = "C:\MsiBuilderProbe\"

CompLocator (base.exe is the key file)
P_COMP_T0              Type 0                                           = <unset>
P_COMP_T1              Type 1                                           = "C:\Program Files\MsiBuilderProbeBase\"
P_COMP_TNULL           Type null                                        = "C:\Program Files\MsiBuilderProbeBase\"
P_COMP_LOWER           Type 1, GUID in lower case                       = "C:\Program Files\MsiBuilderProbeBase\"
P_COMP_T1_SIG          Type 1, Signature base.exe                       = "C:\Program Files\MsiBuilderProbeBase\base.exe"
P_COMP_T1_SIG_WRONG    Type 1, Signature other.exe                      = "C:\Program Files\MsiBuilderProbeBase\base.exe"
P_COMP_T0_SIG          Type 0, Signature base.exe                       = <unset>
P_COMP_MISSING         unknown GUID, Type 1                             = <unset>
P_FILE_UNDER_COMP1     base.exe, DrLocator(CompLocator Type 1, -, 0)    = "C:\Program Files\MsiBuilderProbeBase\base.exe"
P_FILE_UNDER_COMP0     base.exe, DrLocator(CompLocator Type 0, -, 0)    = <unset>

Several AppSearch rows for one property (rows in key order of Signature_)
P_MULTI_MISS_HIT       S_MULTI_A_MISS, S_MULTI_B_HIT (C:\MsiBuilderProbe)          = "C:\MsiBuilderProbe\"
P_MULTI_HIT_HIT        S_MULTI_B_HIT, S_MULTI_C_HIT (C:\MsiBuilderProbe\sub)       = "C:\MsiBuilderProbe\sub\"
P_MULTI_HIT_MISS       S_MULTI_C_HIT, S_MULTI_A_MISS                               = "C:\MsiBuilderProbe\sub\"
P_ORDER_COMPMISS_DR    one signature: CompLocator (unknown GUID) and DrLocator ...\sub = "C:\MsiBuilderProbe\sub\"
P_PRESET               Property row "preset", DrLocator C:\MsiBuilderProbe          = "C:\MsiBuilderProbe\"
```
