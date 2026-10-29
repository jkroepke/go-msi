# ENV-02 / wixl
package wixl.msi: 9728 bytes sha256 9b43707ad8b8839a563062f1e6bfb6818678ca5a48c82b55d1e8662b75692258
CFB major 3, sector 512, 20 directory entries, root CLSID 84 10 0c 00 00 00 00 00 c0 00 00 00 00 00 00 46 (MSI database CLSID: true)
header: minor 0x3e, mini cutoff 4096, 1 FAT sectors, transaction 0

| Stream | Kind | Size | SHA-256 |
| --- | --- | ---: | --- |
| `\005SummaryInformation` | property set | 516 | db311bebecb15ebb |
| `payload.cab` | stream | 106 | 1b3fb9f4832a21b3 |
| `AdminExecuteSequence` | table | 48 | 0996c9397d0eab3e |
| `AdminUISequence` | table | 24 | d8b38a3422164ba1 |
| `AdvtExecuteSequence` | table | 42 | 6b13cbd95d57613a |
| `Component` | table | 12 | 04b0398933c99f88 |
| `Directory` | table | 18 | 3d10460c48964048 |
| `Feature` | table | 16 | a103067fe35f4c7d |
| `FeatureComponents` | table | 4 | 1bce79b721d27ac4 |
| `File` | table | 20 | 512983ca1d62c61d |
| `InstallExecuteSequence` | table | 90 | b14823a1a89addc6 |
| `InstallUISequence` | table | 30 | 2dfd9469822a4e1b |
| `Media` | table | 14 | dd19b3208a43f727 |
| `MsiFileHash` | table | 20 | 00cff2a4e9e4a816 |
| `Property` | table | 28 | 80d58f448a889924 |
| `_Columns` | table | 1328 | c46678afa2837d5a |
| `_StringData` | table | 1614 | 9e14773774f08141 |
| `_StringPool` | table | 836 | 27e2438ce850c926 |
| `_Tables` | table | 66 | 95c6129615bb5822 |

string pool: code page 0, long refs false, 208 entries
  refcount id 1 "Media": stored 6, counted 7
  refcount id 9 "AdminUISequence": stored 3, counted 4
  refcount id 11 "Condition": stored 11, counted 12
  refcount id 13 "CostInitialize": stored 1, counted 5
  refcount id 14 "FileCost": stored 1, counted 4
refcounts differing from the counted references: 51; entries no row uses: 0
summary PID  1: VT 2 1252
summary PID  2: VT 30 "Installation Database"
summary PID  3: VT 30 "MsiSpec ENV-02 wixl"
summary PID  4: VT 30 "MsiSpec"
summary PID  5: VT 30 "Installer"
summary PID  6: VT 30 "This installer database contains the logic and data required to install MsiSpec ENV-02 wixl."
summary PID  7: VT 30 "x64;1033"
summary PID  9: VT 30 "{249BC880-1B70-4D65-915E-D82C38C2AC14}"
summary PID 12: VT 64 FILETIME 01dd67ac9b636080
summary PID 13: VT 64 FILETIME 01dd67ac9b636080
summary PID 14: VT 3 500
summary PID 15: VT 3 2
summary PID 18: VT 30 "msitools 0.106"
summary PID 19: VT 3 2
native open ok; 33 tables, err=<nil>

table `Media` (DiskId 0x2502, LastSequence 0x0104, DiskPrompt 0x1f40, Cabinet 0x1dff, VolumeLabel 0x1d20, Source 0x1d48), 1 rows:
    1 | 1 | NULL | #payload.cab | NULL | NULL

table `AdminUISequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 4 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    ExecuteAction | NULL | 1300
    FileCost | NULL | 900

table `Condition` (Feature_ 0x2d26, Level 0x2502, Condition 0x1dff), 0 rows:

table `Error` (Error 0x2502, Message 0x1f00), 0 rows:

table `ServiceInstall` (ServiceInstall 0x2d48, Name 0x0dff, DisplayName 0x1fff, ServiceType 0x0104, StartType 0x0104, ErrorControl 0x0104, LoadOrderGroup 0x1dff, Dependencies 0x1dff, StartName 0x1dff, Password 0x1dff, Arguments 0x1dff, Component_ 0x0d48, Description 0x1fff), 0 rows:

table `AppSearch` (Property 0x2d48, Signature_ 0x2d48), 0 rows:

table `Property` (Property 0x2d48, Value 0x0f00), 7 rows:
    ALLUSERS | 1
    Manufacturer | MsiSpec
    ProductCode | {0105246E-4D3A-4741-8061-6E5A7DBCBE42}
    ProductLanguage | 1033
    ProductName | MsiSpec ENV-02 wixl
    ProductVersion | 1.0.0
    UpgradeCode | {4E72965B-6375-48E7-93A6-A65B0CE630CA}

table `Upgrade` (UpgradeCode 0x2d26, VersionMin 0x3d14, VersionMax 0x3d14, Language 0x3dff, Attributes 0x2104, Remove 0x1dff, ActionProperty 0x0d48), 0 rows:

table `Shortcut` (Shortcut 0x2d48, Directory_ 0x0d48, Name 0x0f80, Component_ 0x0d48, Target 0x0d48, Arguments 0x1dff, Description 0x1fff, Hotkey 0x1502, Icon_ 0x1d48, IconIndex 0x1502, ShowCmd 0x1502, WkDir 0x1d48, DisplayResourceDLL 0x1dff, DisplayResourceId 0x1502, DescriptionResourceDLL 0x1dff, DescriptionResourceId 0x1502), 0 rows:

table `InstallExecuteSequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 15 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    FileCost | NULL | 900
    InstallFiles | NULL | 4000
    InstallFinalize | NULL | 6600
    InstallInitialize | NULL | 1500
    InstallValidate | NULL | 1400
    ProcessComponents | NULL | 1600
    PublishFeatures | NULL | 6300
    PublishProduct | NULL | 6400
    RegisterProduct | NULL | 6100
    RegisterUser | NULL | 6000
    RemoveFiles | NULL | 3500
    UnpublishFeatures | NULL | 1800
    ValidateProductID | NULL | 700

table `Registry` (Registry 0x2d48, Root 0x0502, Key 0x0fff, Name 0x1fff, Value 0x1f00, Component_ 0x0d48), 0 rows:

table `AdvtExecuteSequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 7 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    InstallFinalize | NULL | 6600
    InstallInitialize | NULL | 1500
    InstallValidate | NULL | 1400
    PublishFeatures | NULL | 6300
    PublishProduct | NULL | 6400

table `ActionText` (Action 0x2d48, Description 0x1fff, Template 0x1fff), 0 rows:

table `FeatureComponents` (Feature_ 0x2d26, Component_ 0x2d48), 1 rows:
    Main | Payload

table `File` (File 0x2d48, Component_ 0x0d48, FileName 0x0fff, FileSize 0x0104, Version 0x1d48, Language 0x1d14, Attributes 0x1502, Sequence 0x0104), 1 rows:
    hello.txt | Payload | hello.txt | 24 | NULL | NULL | 512 | 1

table `Directory` (Directory 0x2d48, Directory_Parent 0x1d48, DefaultDir 0x0fff), 3 rows:
    INSTALLFOLDER | ProgramFiles64Folder | MsiSpecEnv02Wixl
    ProgramFiles64Folder | TARGETDIR | .
    TARGETDIR | NULL | SourceDir

table `IniFile` (IniFile 0x2d48, FileName 0x0fff, DirProperty 0x1d48, Section 0x0fff, Key 0x0fff, Value 0x0fff, Action 0x0502, Component_ 0x0d48), 0 rows:

table `RemoveFile` (FileKey 0x2d48, Component_ 0x0d48, FileName 0x1fff, DirProperty 0x0d48, InstallMode 0x0502), 0 rows:

table `RemoveIniFile` (RemoveIniFile 0x2d48, FileName 0x0fff, DirProperty 0x1d48, Section 0x0fff, Key 0x0fff, Value 0x1fff, Action 0x0502, Component_ 0x0d48), 0 rows:

table `LaunchCondition` (Condition 0x2dff, Description 0x0fff), 0 rows:

table `Signature` (Signature 0x2d48, FileName 0x0dff, MinVersion 0x1d14, MaxVersion 0x1d14, MinSize 0x1104, MaxSize 0x1104, MinDate 0x1104, MaxDate 0x1104, Languages 0x1dff), 0 rows:

table `Environment` (Environment 0x2d48, Name 0x0f40, Value 0x1fff, Component_ 0x0d48), 0 rows:

table `InstallUISequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 5 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    ExecuteAction | NULL | 1300
    FileCost | NULL | 900
    ValidateProductID | NULL | 700

table `Component` (Component 0x2d48, ComponentId 0x1d26, Directory_ 0x0d48, Attributes 0x0502, Condition 0x1dff, KeyPath 0x1d48), 1 rows:
    Payload | {820E3071-DA44-43F3-AA51-6FB1A1CC58F8} | INSTALLFOLDER | 256 | NULL | hello.txt

table `ServiceControl` (ServiceControl 0x2d48, Name 0x0fff, Event 0x0502, Arguments 0x1fff, Wait 0x1502, Component_ 0x0d48), 0 rows:

table `Feature` (Feature 0x2d26, Feature_Parent 0x1d26, Title 0x1f40, Description 0x1fff, Display 0x1502, Level 0x0502, Directory_ 0x1d48, Attributes 0x0502), 1 rows:
    Main | NULL | NULL | NULL | 2 | 1 | NULL | 0

table `Binary` (Name 0x2d48, Data 0x0900), 0 rows:

table `CustomAction` (Action 0x2d48, Type 0x0502, Source 0x1d48, Target 0x1dff, ExtendedType 0x1104), 0 rows:

table `RegLocator` (Signature_ 0x2d48, Root 0x0502, Key 0x0dff, Name 0x1dff, Type 0x1502), 0 rows:

table `Icon` (Name 0x2d48, Data 0x0900), 0 rows:

table `CreateFolder` (Directory_ 0x2d48, Component_ 0x2d48), 0 rows:

table `AdminExecuteSequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 8 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    FileCost | NULL | 900
    InstallAdminPackage | NULL | 3900
    InstallFiles | NULL | 4000
    InstallFinalize | NULL | 6600
    InstallInitialize | NULL | 1500
    InstallValidate | NULL | 1400

table `MsiFileHash` (File_ 0x2d48, Options 0x0502, HashPart1 0x0104, HashPart2 0x0104, HashPart3 0x0104, HashPart4 0x0104), 1 rows:
    hello.txt | 0 | -243489559 | -522936822 | -1715805770 | 1034370382

cabinet #payload.cab: 106 bytes, header "MSCF", set 0/0, 1 folders, 1 files, flags 0x0
  folder 0: 1 data blocks, compression 0x0001
  msitest reader: file "hello.txt", 24 bytes, attrib 0x20, payload match true
  expand.exe: hello.txt payload match true (err=<nil>)
msiexec /i C:\Users\Administrator\GolandProjects\msi-builder-env02\docs\spec\msitest\env02\wixl.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\3fc8a3.msi
  log: Product: MsiSpec ENV-02 wixl -- Installation completed successfully.
  log: MainEngineThread is returning 0
  installed file C:\Program Files\MsiSpecEnv02Wixl\hello.txt: ok sha 66dd4842edbfca27cdea8576a64e5db16f76a382ae5bcb0938e112b1a540f5bd
msiexec /x C:\Users\Administrator\GolandProjects\msi-builder-env02\docs\spec\msitest\env02\wixl.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec ENV-02 wixl -- Removal completed successfully.
  log: MainEngineThread is returning 0
install/uninstall cycle ok: true
