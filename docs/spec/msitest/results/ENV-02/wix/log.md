# ENV-02 / wix
wix --version: 5.0.2+aa65968c
wix build -arch x64 (err=<nil>): 
package wix.msi: 32768 bytes sha256 d94b95107ce1bfca5da6e669e5c546c9d3338abe625d42f25a38bc4316db7714
CFB major 4, sector 4096, 32 directory entries, root CLSID 84 10 0c 00 00 00 00 00 c0 00 00 00 00 00 00 46 (MSI database CLSID: true)
header: minor 0x3e, mini cutoff 4096, 1 FAT sectors, transaction 0

| Stream | Kind | Size | SHA-256 |
| --- | --- | ---: | --- |
| `\005SummaryInformation` | property set | 520 | 292386647c5ae69e |
| `payload.cab` | stream | 122 | 5a1d23b8fd948159 |
| `AdminExecuteSequence` | table | 48 | 13b29035c9362e43 |
| `AdminUISequence` | table | 24 | 59f829718c9f61be |
| `AdvtExecuteSequence` | table | 42 | 58068b8ea20adf27 |
| `Component` | table | 12 | 7bab049d76c25d0f |
| `Directory` | table | 18 | 18da9a30244f05db |
| `Feature` | table | 16 | df56b89a01e608df |
| `FeatureComponents` | table | 4 | 603b3f537c0c4a51 |
| `File` | table | 20 | 1ca09621fa2a6fe9 |
| `InstallExecuteSequence` | table | 126 | 308c68ecc1efd2a3 |
| `InstallUISequence` | table | 48 | 548e365e7e142d48 |
| `LaunchCondition` | table | 4 | 19f0ff15f6edf59a |
| `Media` | table | 14 | 6951b887712c1b7d |
| `MsiFileHash` | table | 20 | 9fb640bba27e0112 |
| `Property` | table | 32 | 40aefd884d95e62a |
| `Upgrade` | table | 32 | d1b6390376c6a2ca |
| `_Columns` | table | 600 | c101eb5e9d990945 |
| `_StringData` | table | 6641 | 97a3f15996a8012f |
| `_StringPool` | table | 780 | 163918353fa9073a |
| `_Tables` | table | 32 | 17c9de9885a30adb |
| `_Validation` | table | 1848 | 850d115b9030fbf6 |

string pool: code page 65001, long refs false, 194 entries
refcounts differing from the counted references: 0; entries no row uses: 0
summary PID  1: VT 2 1252
summary PID  2: VT 30 "Installation Database"
summary PID  3: VT 30 "MsiSpec ENV-02 WiX"
summary PID  4: VT 30 "MsiSpec"
summary PID  5: VT 30 "Installer"
summary PID  6: VT 30 "This installer database contains the logic and data required to install MsiSpec ENV-02 WiX."
summary PID  7: VT 30 "x64;1033"
summary PID  9: VT 30 "{7987FBAC-BB40-4745-8280-47A240F226CA}"
summary PID 12: VT 64 FILETIME 01dd67ae03ff0d00
summary PID 13: VT 64 FILETIME 01dd67ae03ff0d00
summary PID 14: VT 3 500
summary PID 15: VT 3 2
summary PID 18: VT 30 "WiX Toolset (5.0.2.0)"
summary PID 19: VT 3 2
native open ok; 16 tables, err=<nil>

table `Property` (Property 0x2d48, Value 0x0f00), 8 rows:
    ALLUSERS | 1
    Manufacturer | MsiSpec
    ProductCode | {777A7C99-BD8F-4678-B757-60713E587C66}
    ProductLanguage | 1033
    ProductName | MsiSpec ENV-02 WiX
    ProductVersion | 1.0.0
    SecureCustomProperties | WIX_DOWNGRADE_DETECTED;WIX_UPGRADE_DETECTED
    UpgradeCode | {F96131A9-2707-4B9F-915F-517BD8D818DA}

table `Media` (DiskId 0x2502, LastSequence 0x0104, DiskPrompt 0x1f40, Cabinet 0x1dff, VolumeLabel 0x1d20, Source 0x1d48), 1 rows:
    1 | 1 | NULL | #payload.cab | NULL | NULL

table `File` (File 0x2d48, Component_ 0x0d48, FileName 0x0fff, FileSize 0x0104, Version 0x1d48, Language 0x1d14, Attributes 0x1502, Sequence 0x0104), 1 rows:
    hello.txt | Payload | hello.txt | 24 | NULL | NULL | 512 | 1

table `Component` (Component 0x2d48, ComponentId 0x1d26, Directory_ 0x0d48, Attributes 0x0502, Condition 0x1dff, KeyPath 0x1d48), 1 rows:
    Payload | {FC8D7D0E-C451-4563-88ED-D16C726E6E4E} | INSTALLFOLDER | 256 | NULL | hello.txt

table `Directory` (Directory 0x2d48, Directory_Parent 0x1d48, DefaultDir 0x0fff), 3 rows:
    INSTALLFOLDER | ProgramFiles64Folder | iugsytnf|MsiSpecEnv02Wix
    ProgramFiles64Folder | TARGETDIR | PFiles64
    TARGETDIR | NULL | SourceDir

table `Feature` (Feature 0x2d26, Feature_Parent 0x1d26, Title 0x1f40, Description 0x1fff, Display 0x1502, Level 0x0502, Directory_ 0x1d48, Attributes 0x0502), 1 rows:
    Main | NULL | NULL | NULL | 2 | 1 | NULL | 0

table `Upgrade` (UpgradeCode 0x2d26, VersionMin 0x3d14, VersionMax 0x3d14, Language 0x3dff, Attributes 0x2104, Remove 0x1dff, ActionProperty 0x0d48), 2 rows:
    {F96131A9-2707-4B9F-915F-517BD8D818DA} | 1.0.0 | NULL | 1033 | 2 | NULL | WIX_DOWNGRADE_DETECTED
    {F96131A9-2707-4B9F-915F-517BD8D818DA} | NULL | 1.0.0 | 1033 | 1 | NULL | WIX_UPGRADE_DETECTED

table `LaunchCondition` (Condition 0x2dff, Description 0x0fff), 1 rows:
    NOT WIX_DOWNGRADE_DETECTED | A newer version of [ProductName] is already installed.

table `FeatureComponents` (Feature_ 0x2d26, Component_ 0x2d48), 1 rows:
    Main | Payload

table `AdminUISequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 4 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    ExecuteAction | NULL | 1300
    FileCost | NULL | 900

table `AdminExecuteSequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 8 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    FileCost | NULL | 900
    InstallAdminPackage | NULL | 3900
    InstallFiles | NULL | 4000
    InstallFinalize | NULL | 6600
    InstallInitialize | NULL | 1500
    InstallValidate | NULL | 1400

table `AdvtExecuteSequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 7 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    InstallFinalize | NULL | 6600
    InstallInitialize | NULL | 1500
    InstallValidate | NULL | 1400
    PublishFeatures | NULL | 6300
    PublishProduct | NULL | 6400

table `InstallUISequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 8 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    ExecuteAction | NULL | 1300
    FileCost | NULL | 900
    FindRelatedProducts | NULL | 25
    LaunchConditions | NULL | 100
    MigrateFeatureStates | NULL | 1200
    ValidateProductID | NULL | 700

table `InstallExecuteSequence` (Action 0x2d48, Condition 0x1dff, Sequence 0x1502), 21 rows:
    CostFinalize | NULL | 1000
    CostInitialize | NULL | 800
    CreateFolders | NULL | 3700
    FileCost | NULL | 900
    FindRelatedProducts | NULL | 25
    InstallFiles | NULL | 4000
    InstallFinalize | NULL | 6600
    InstallInitialize | NULL | 1500
    InstallValidate | NULL | 1400
    LaunchConditions | NULL | 100
    MigrateFeatureStates | NULL | 1200
    ProcessComponents | NULL | 1600
    PublishFeatures | NULL | 6300
    PublishProduct | NULL | 6400
    RegisterProduct | NULL | 6100
    RegisterUser | NULL | 6000
    RemoveExistingProducts | NULL | 1401
    RemoveFiles | NULL | 3500
    RemoveFolders | NULL | 3600
    UnpublishFeatures | NULL | 1800
    ValidateProductID | NULL | 700

table `MsiFileHash` (File_ 0x2d48, Options 0x0502, HashPart1 0x0104, HashPart2 0x0104, HashPart3 0x0104, HashPart4 0x0104), 1 rows:
    hello.txt | 0 | -243489559 | -522936822 | -1715805770 | 1034370382

table `_Validation` (Table 0x2d20, Column 0x2d20, Nullable 0x0d04, MinValue 0x1104, MaxValue 0x1104, KeyTable 0x1dff, KeyColumn 0x1502, Category 0x1d20, Set 0x1dff, Description 0x1dff), 77 rows:
    AdminExecuteSequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL.
    AdminExecuteSequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse.If the expression syntax is invalid, the engine will terminate, returning iesBadActionData.
    AdminExecuteSequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed.  Leave blank to suppress action.
    AdminUISequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL.
    AdminUISequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse.If the expression syntax is invalid, the engine will terminate, returning iesBadActionData.
    AdminUISequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed.  Leave blank to suppress action.
    AdvtExecuteSequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL.
    AdvtExecuteSequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse.If the expression syntax is invalid, the engine will terminate, returning iesBadActionData.
    AdvtExecuteSequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed.  Leave blank to suppress action.
    Component | Attributes | N | NULL | NULL | NULL | NULL | NULL | NULL | Remote execution option, one of irsEnum
    Component | Component | N | NULL | NULL | NULL | NULL | Identifier | NULL | Primary key used to identify a particular component record.
    Component | ComponentId | Y | NULL | NULL | NULL | NULL | Guid | NULL | A string GUID unique to this component, version, and language.
    Component | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | A conditional statement that will disable this component if the specified condition evaluates to the 'True' state. If a component is disabled, it will not be installed, regardless of the 'Action' state associated with the component.
    Component | Directory_ | N | NULL | NULL | Directory | 1 | Identifier | NULL | Required key of a Directory table record. This is actually a property name whose value contains the actual path, set either by the AppSearch action or with the default setting obtained from the Directory table.
    Component | KeyPath | Y | NULL | NULL | File;Registry;ODBCDataSource | 1 | Identifier | NULL | Either the primary key into the File table, Registry table, or ODBCDataSource table. This extract path is stored when the component is installed, and is used to detect the presence of the component and to return the path to it.
    Directory | DefaultDir | N | NULL | NULL | NULL | NULL | DefaultDir | NULL | The default sub-path under parent's path.
    Directory | Directory | N | NULL | NULL | NULL | NULL | Identifier | NULL | Unique identifier for directory entry, primary key. If a property by this name is defined, it contains the full path to the directory.
    Directory | Directory_Parent | Y | NULL | NULL | Directory | 1 | Identifier | NULL | Reference to the entry in this table specifying the default parent directory. A record parented to itself or with a Null parent represents a root of the install tree.
    Feature | Attributes | N | NULL | NULL | NULL | NULL | NULL | 0;1;2;4;5;6;8;9;10;16;17;18;20;21;22;24;25;26;32;33;34;36;37;38;48;49;50;52;53;54 | Feature attributes
    Feature | Description | Y | NULL | NULL | NULL | NULL | Text | NULL | Longer descriptive text describing a visible feature item.
    Feature | Directory_ | Y | NULL | NULL | Directory | 1 | UpperCase | NULL | The name of the Directory that can be configured by the UI. A non-null value will enable the browse button.
    Feature | Display | Y | 0 | 32767 | NULL | NULL | NULL | NULL | Numeric sort order, used to force a specific display ordering.
    Feature | Feature | N | NULL | NULL | NULL | NULL | Identifier | NULL | Primary key used to identify a particular feature record.
    Feature | Feature_Parent | Y | NULL | NULL | Feature | 1 | Identifier | NULL | Optional key of a parent record in the same table. If the parent is not selected, then the record will not be installed. Null indicates a root item.
    Feature | Level | N | 0 | 32767 | NULL | NULL | NULL | NULL | The install level at which record will be initially selected. An install level of 0 will disable an item and prevent its display.
    Feature | Title | Y | NULL | NULL | NULL | NULL | Text | NULL | Short text identifying a visible feature item.
    FeatureComponents | Component_ | N | NULL | NULL | Component | 1 | Identifier | NULL | Foreign key into Component table.
    FeatureComponents | Feature_ | N | NULL | NULL | Feature | 1 | Identifier | NULL | Foreign key into Feature table.
    File | Attributes | Y | 0 | 32767 | NULL | NULL | NULL | NULL | Integer containing bit flags representing file attributes (with the decimal value of each bit position in parentheses)
    File | Component_ | N | NULL | NULL | Component | 1 | Identifier | NULL | Foreign key referencing Component that controls the file.
    File | File | N | NULL | NULL | NULL | NULL | Identifier | NULL | Primary key, non-localized token, must match identifier in cabinet.  For uncompressed files, this field is ignored.
    File | FileName | N | NULL | NULL | NULL | NULL | Filename | NULL | File name used for installation, may be localized.  This may contain a "short name|long name" pair.
    File | FileSize | N | 0 | 2147483647 | NULL | NULL | NULL | NULL | Size of file in bytes (long integer).
    File | Language | Y | NULL | NULL | NULL | NULL | Language | NULL | List of decimal language Ids, comma-separated if more than one.
    File | Sequence | N | 1 | 2147483647 | NULL | NULL | NULL | NULL | Sequence with respect to the media images; order must track cabinet order.
    File | Version | Y | NULL | NULL | File | 1 | Version | NULL | Version string for versioned files;  Blank for unversioned files.
    InstallExecuteSequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL.
    InstallExecuteSequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse. If the expression syntax is invalid, the engine will terminate, returning iesBadActionData.
    InstallExecuteSequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed. Leave blank to suppress action.
    InstallUISequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL.
    InstallUISequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse. If the expression syntax is invalid, the engine will terminate, returning iesBadActionData.
    InstallUISequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed. Leave blank to suppress action.
    LaunchCondition | Condition | N | NULL | NULL | NULL | NULL | Condition | NULL | Expression which must evaluate to TRUE in order for install to commence.
    LaunchCondition | Description | N | NULL | NULL | NULL | NULL | Formatted | NULL | Localizable text to display when condition fails and install must abort.
    Media | Cabinet | Y | NULL | NULL | NULL | NULL | Cabinet | NULL | If some or all of the files stored on the media are compressed in a cabinet, the name of that cabinet.
    Media | DiskId | N | 1 | 32767 | NULL | NULL | NULL | NULL | Primary key, integer to determine sort order for table.
    Media | DiskPrompt | Y | NULL | NULL | NULL | NULL | Text | NULL | Disk name: the visible text actually printed on the disk.  This will be used to prompt the user when this disk needs to be inserted.
    Media | LastSequence | N | 0 | 2147483647 | NULL | NULL | NULL | NULL | File sequence number for the last file for this media.
    Media | Source | Y | NULL | NULL | NULL | NULL | Property | NULL | The property defining the location of the cabinet file.
    Media | VolumeLabel | Y | NULL | NULL | NULL | NULL | Text | NULL | The label attributed to the volume.
    MsiFileHash | File_ | N | NULL | NULL | File | 1 | Identifier | NULL | Primary key, foreign key into File table referencing file with this hash
    MsiFileHash | HashPart1 | N | NULL | NULL | NULL | NULL | NULL | NULL | Size of file in bytes (long integer).
    MsiFileHash | HashPart2 | N | NULL | NULL | NULL | NULL | NULL | NULL | Size of file in bytes (long integer).
    MsiFileHash | HashPart3 | N | NULL | NULL | NULL | NULL | NULL | NULL | Size of file in bytes (long integer).
    MsiFileHash | HashPart4 | N | NULL | NULL | NULL | NULL | NULL | NULL | Size of file in bytes (long integer).
    MsiFileHash | Options | N | 0 | 32767 | NULL | NULL | NULL | NULL | Various options and attributes for this hash.
    Property | Property | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of property, uppercase if settable by launcher or loader.
    Property | Value | N | NULL | NULL | NULL | NULL | Text | NULL | String value for property.  Never null or empty.
    Upgrade | ActionProperty | N | NULL | NULL | NULL | NULL | UpperCase | NULL | The property to set when a product in this set is found.
    Upgrade | Attributes | N | 0 | 2147483647 | NULL | NULL | NULL | NULL | The attributes of this product set.
    Upgrade | Language | Y | NULL | NULL | NULL | NULL | Language | NULL | A comma-separated list of languages for either products in this set or products not in this set.
    Upgrade | Remove | Y | NULL | NULL | NULL | NULL | Formatted | NULL | The list of features to remove when uninstalling a product from this set.  The default is "ALL".
    Upgrade | UpgradeCode | N | NULL | NULL | NULL | NULL | Guid | NULL | The UpgradeCode GUID belonging to the products in this set.
    Upgrade | VersionMax | Y | NULL | NULL | NULL | NULL | Text | NULL | The maximum ProductVersion of the products in this set.  The set may or may not include products with this particular version.
    Upgrade | VersionMin | Y | NULL | NULL | NULL | NULL | Text | NULL | The minimum ProductVersion of the products in this set.  The set may or may not include products with this particular version.
    _SummaryInformation | PropertyId | N | NULL | NULL | NULL | NULL | NULL | NULL | NULL
    _SummaryInformation | Value | N | NULL | NULL | NULL | NULL | NULL | NULL | NULL
    _Validation | Category | Y | NULL | NULL | NULL | NULL | NULL | Text;Formatted;Template;Condition;Guid;Path;Version;Language;Identifier;Binary;UpperCase;LowerCase;Filename;Paths;AnyPath;WildCardFilename;RegPath;CustomSource;Property;Cabinet;Shortcut;FormattedSDDLText;Integer;DoubleInteger;TimeDate;DefaultDir | String category
    _Validation | Column | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of column
    _Validation | Description | Y | NULL | NULL | NULL | NULL | Text | NULL | Description of column
    _Validation | KeyColumn | Y | 1 | 32 | NULL | NULL | NULL | NULL | Column to which foreign key connects
    _Validation | KeyTable | Y | NULL | NULL | NULL | NULL | Identifier | NULL | For foreign key, Name of table to which data must link
    _Validation | MaxValue | Y | -2147483647 | 2147483647 | NULL | NULL | NULL | NULL | Maximum value allowed
    _Validation | MinValue | Y | -2147483647 | 2147483647 | NULL | NULL | NULL | NULL | Minimum value allowed
    _Validation | Nullable | N | NULL | NULL | NULL | NULL | NULL | Y;N | Whether the column is nullable
    _Validation | Set | Y | NULL | NULL | NULL | NULL | Text | NULL | Set of values that are permitted
    _Validation | Table | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of table

cabinet #payload.cab: 122 bytes, header "MSCF", set 0/0, 1 folders, 1 files, flags 0x0
  folder 0: 1 data blocks, compression 0x1203
  msitest reader: compression 3 unsupported
  expand.exe: hello.txt payload match true (err=<nil>)
msiexec /i C:\Users\Administrator\GolandProjects\msi-builder-env02\docs\spec\msitest\results\ENV-02\wix\wix.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\3fc89c.msi
  log: Product: MsiSpec ENV-02 WiX -- Installation completed successfully.
  log: MainEngineThread is returning 0
  installed file C:\Program Files\MsiSpecEnv02Wix\hello.txt: ok sha 66dd4842edbfca27cdea8576a64e5db16f76a382ae5bcb0938e112b1a540f5bd
msiexec /x C:\Users\Administrator\GolandProjects\msi-builder-env02\docs\spec\msitest\results\ENV-02\wix\wix.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec ENV-02 WiX -- Removal completed successfully.
  log: MainEngineThread is returning 0
install/uninstall cycle ok: true
