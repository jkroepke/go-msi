# ENV-02 / compare
Differences between wix.msi (A) and wixl.msi (B); identical entries are left out.

streams (size sha):
  property set \005SummaryInformation: A 520 292386647c5ae69e / B 516 db311bebecb15ebb
  stream payload.cab: A 122 5a1d23b8fd948159 / B 106 1b3fb9f4832a21b3
  table AdminExecuteSequence: A 48 13b29035c9362e43 / B 48 0996c9397d0eab3e
  table AdminUISequence: A 24 59f829718c9f61be / B 24 d8b38a3422164ba1
  table AdvtExecuteSequence: A 42 58068b8ea20adf27 / B 42 6b13cbd95d57613a
  table Component: A 12 7bab049d76c25d0f / B 12 04b0398933c99f88
  table Directory: A 18 18da9a30244f05db / B 18 3d10460c48964048
  table Feature: A 16 df56b89a01e608df / B 16 a103067fe35f4c7d
  table FeatureComponents: A 4 603b3f537c0c4a51 / B 4 1bce79b721d27ac4
  table File: A 20 1ca09621fa2a6fe9 / B 20 512983ca1d62c61d
  table InstallExecuteSequence: A 126 308c68ecc1efd2a3 / B 90 b14823a1a89addc6
  table InstallUISequence: A 48 548e365e7e142d48 / B 30 2dfd9469822a4e1b
  only A: table LaunchCondition = 4 19f0ff15f6edf59a
  table Media: A 14 6951b887712c1b7d / B 14 dd19b3208a43f727
  table MsiFileHash: A 20 9fb640bba27e0112 / B 20 00cff2a4e9e4a816
  table Property: A 32 40aefd884d95e62a / B 28 80d58f448a889924
  only A: table Upgrade = 32 d1b6390376c6a2ca
  table _Columns: A 600 c101eb5e9d990945 / B 1328 c46678afa2837d5a
  table _StringData: A 6641 97a3f15996a8012f / B 1614 9e14773774f08141
  table _StringPool: A 780 163918353fa9073a / B 836 27e2438ce850c926
  table _Tables: A 32 17c9de9885a30adb / B 66 95c6129615bb5822
  only A: table _Validation = 1848 850d115b9030fbf6

table columns:
  only B: ActionText = Action 0x2d48, Description 0x1fff, Template 0x1fff
  only B: AppSearch = Property 0x2d48, Signature_ 0x2d48
  only B: Binary = Name 0x2d48, Data 0x0900
  only B: Condition = Feature_ 0x2d26, Level 0x2502, Condition 0x1dff
  only B: CreateFolder = Directory_ 0x2d48, Component_ 0x2d48
  only B: CustomAction = Action 0x2d48, Type 0x0502, Source 0x1d48, Target 0x1dff, ExtendedType 0x1104
  only B: Environment = Environment 0x2d48, Name 0x0f40, Value 0x1fff, Component_ 0x0d48
  only B: Error = Error 0x2502, Message 0x1f00
  only B: Icon = Name 0x2d48, Data 0x0900
  only B: IniFile = IniFile 0x2d48, FileName 0x0fff, DirProperty 0x1d48, Section 0x0fff, Key 0x0fff, Value 0x0fff, Action 0x0502, Component_ 0x0d48
  only B: RegLocator = Signature_ 0x2d48, Root 0x0502, Key 0x0dff, Name 0x1dff, Type 0x1502
  only B: Registry = Registry 0x2d48, Root 0x0502, Key 0x0fff, Name 0x1fff, Value 0x1f00, Component_ 0x0d48
  only B: RemoveFile = FileKey 0x2d48, Component_ 0x0d48, FileName 0x1fff, DirProperty 0x0d48, InstallMode 0x0502
  only B: RemoveIniFile = RemoveIniFile 0x2d48, FileName 0x0fff, DirProperty 0x1d48, Section 0x0fff, Key 0x0fff, Value 0x1fff, Action 0x0502, Component_ 0x0d48
  only B: ServiceControl = ServiceControl 0x2d48, Name 0x0fff, Event 0x0502, Arguments 0x1fff, Wait 0x1502, Component_ 0x0d48
  only B: ServiceInstall = ServiceInstall 0x2d48, Name 0x0dff, DisplayName 0x1fff, ServiceType 0x0104, StartType 0x0104, ErrorControl 0x0104, LoadOrderGroup 0x1dff, Dependencies 0x1dff, StartName 0x1dff, Password 0x1dff, Arguments 0x1dff, Component_ 0x0d48, Description 0x1fff
  only B: Shortcut = Shortcut 0x2d48, Directory_ 0x0d48, Name 0x0f80, Component_ 0x0d48, Target 0x0d48, Arguments 0x1dff, Description 0x1fff, Hotkey 0x1502, Icon_ 0x1d48, IconIndex 0x1502, ShowCmd 0x1502, WkDir 0x1d48, DisplayResourceDLL 0x1dff, DisplayResourceId 0x1502, DescriptionResourceDLL 0x1dff, DescriptionResourceId 0x1502
  only B: Signature = Signature 0x2d48, FileName 0x0dff, MinVersion 0x1d14, MaxVersion 0x1d14, MinSize 0x1104, MaxSize 0x1104, MinDate 0x1104, MaxDate 0x1104, Languages 0x1dff
  only A: _Validation = Table 0x2d20, Column 0x2d20, Nullable 0x0d04, MinValue 0x1104, MaxValue 0x1104, KeyTable 0x1dff, KeyColumn 0x1502, Category 0x1d20, Set 0x1dff, Description 0x1dff

summary information:
  PID  3: A VT 30 "MsiSpec ENV-02 WiX" / B VT 30 "MsiSpec ENV-02 wixl"
  PID  6: A VT 30 "This installer database contains the logic and data required to install MsiSpec ENV-02 WiX." / B VT 30 "This installer database contains the logic and data required to install MsiSpec ENV-02 wixl."
  PID  9: A VT 30 "{7987FBAC-BB40-4745-8280-47A240F226CA}" / B VT 30 "{249BC880-1B70-4D65-915E-D82C38C2AC14}"
  PID 12: A VT 64 FILETIME 01dd67ae03ff0d00 / B VT 64 FILETIME 01dd67ac9b636080
  PID 13: A VT 64 FILETIME 01dd67ae03ff0d00 / B VT 64 FILETIME 01dd67ac9b636080
  PID 18: A VT 30 "WiX Toolset (5.0.2.0)" / B VT 30 "msitools 0.106"

rows:
  only B: Component: Payload | {820E3071-DA44-43F3-AA51-6FB1A1CC58F8} | INSTALLFOLDER | 256 | NULL | hello.txt = row
  only A: Component: Payload | {FC8D7D0E-C451-4563-88ED-D16C726E6E4E} | INSTALLFOLDER | 256 | NULL | hello.txt = row
  only B: Directory: INSTALLFOLDER | ProgramFiles64Folder | MsiSpecEnv02Wixl = row
  only A: Directory: INSTALLFOLDER | ProgramFiles64Folder | iugsytnf|MsiSpecEnv02Wix = row
  only B: Directory: ProgramFiles64Folder | TARGETDIR | . = row
  only A: Directory: ProgramFiles64Folder | TARGETDIR | PFiles64 = row
  only A: InstallExecuteSequence: CreateFolders | NULL | 3700 = row
  only A: InstallExecuteSequence: FindRelatedProducts | NULL | 25 = row
  only A: InstallExecuteSequence: LaunchConditions | NULL | 100 = row
  only A: InstallExecuteSequence: MigrateFeatureStates | NULL | 1200 = row
  only A: InstallExecuteSequence: RemoveExistingProducts | NULL | 1401 = row
  only A: InstallExecuteSequence: RemoveFolders | NULL | 3600 = row
  only A: InstallUISequence: FindRelatedProducts | NULL | 25 = row
  only A: InstallUISequence: LaunchConditions | NULL | 100 = row
  only A: InstallUISequence: MigrateFeatureStates | NULL | 1200 = row
  only A: LaunchCondition: NOT WIX_DOWNGRADE_DETECTED | A newer version of [ProductName] is already installed. = row
  only B: Property: ProductCode | {0105246E-4D3A-4741-8061-6E5A7DBCBE42} = row
  only A: Property: ProductCode | {777A7C99-BD8F-4678-B757-60713E587C66} = row
  only A: Property: ProductName | MsiSpec ENV-02 WiX = row
  only B: Property: ProductName | MsiSpec ENV-02 wixl = row
  only A: Property: SecureCustomProperties | WIX_DOWNGRADE_DETECTED;WIX_UPGRADE_DETECTED = row
  only B: Property: UpgradeCode | {4E72965B-6375-48E7-93A6-A65B0CE630CA} = row
  only A: Property: UpgradeCode | {F96131A9-2707-4B9F-915F-517BD8D818DA} = row
  only A: Upgrade: {F96131A9-2707-4B9F-915F-517BD8D818DA} | 1.0.0 | NULL | 1033 | 2 | NULL | WIX_DOWNGRADE_DETECTED = row
  only A: Upgrade: {F96131A9-2707-4B9F-915F-517BD8D818DA} | NULL | 1.0.0 | 1033 | 1 | NULL | WIX_UPGRADE_DETECTED = row
  only A: _Validation: AdminExecuteSequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL. = row
  only A: _Validation: AdminExecuteSequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse.If the expression syntax is invalid, the engine will terminate, returning iesBadActionData. = row
  only A: _Validation: AdminExecuteSequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed.  Leave blank to suppress action. = row
  only A: _Validation: AdminUISequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL. = row
  only A: _Validation: AdminUISequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse.If the expression syntax is invalid, the engine will terminate, returning iesBadActionData. = row
  only A: _Validation: AdminUISequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed.  Leave blank to suppress action. = row
  only A: _Validation: AdvtExecuteSequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL. = row
  only A: _Validation: AdvtExecuteSequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse.If the expression syntax is invalid, the engine will terminate, returning iesBadActionData. = row
  only A: _Validation: AdvtExecuteSequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed.  Leave blank to suppress action. = row
  only A: _Validation: Component | Attributes | N | NULL | NULL | NULL | NULL | NULL | NULL | Remote execution option, one of irsEnum = row
  only A: _Validation: Component | Component | N | NULL | NULL | NULL | NULL | Identifier | NULL | Primary key used to identify a particular component record. = row
  only A: _Validation: Component | ComponentId | Y | NULL | NULL | NULL | NULL | Guid | NULL | A string GUID unique to this component, version, and language. = row
  only A: _Validation: Component | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | A conditional statement that will disable this component if the specified condition evaluates to the 'True' state. If a component is disabled, it will not be installed, regardless of the 'Action' state associated with the component. = row
  only A: _Validation: Component | Directory_ | N | NULL | NULL | Directory | 1 | Identifier | NULL | Required key of a Directory table record. This is actually a property name whose value contains the actual path, set either by the AppSearch action or with the default setting obtained from the Directory table. = row
  only A: _Validation: Component | KeyPath | Y | NULL | NULL | File;Registry;ODBCDataSource | 1 | Identifier | NULL | Either the primary key into the File table, Registry table, or ODBCDataSource table. This extract path is stored when the component is installed, and is used to detect the presence of the component and to return the path to it. = row
  only A: _Validation: Directory | DefaultDir | N | NULL | NULL | NULL | NULL | DefaultDir | NULL | The default sub-path under parent's path. = row
  only A: _Validation: Directory | Directory | N | NULL | NULL | NULL | NULL | Identifier | NULL | Unique identifier for directory entry, primary key. If a property by this name is defined, it contains the full path to the directory. = row
  only A: _Validation: Directory | Directory_Parent | Y | NULL | NULL | Directory | 1 | Identifier | NULL | Reference to the entry in this table specifying the default parent directory. A record parented to itself or with a Null parent represents a root of the install tree. = row
  only A: _Validation: Feature | Attributes | N | NULL | NULL | NULL | NULL | NULL | 0;1;2;4;5;6;8;9;10;16;17;18;20;21;22;24;25;26;32;33;34;36;37;38;48;49;50;52;53;54 | Feature attributes = row
  only A: _Validation: Feature | Description | Y | NULL | NULL | NULL | NULL | Text | NULL | Longer descriptive text describing a visible feature item. = row
  only A: _Validation: Feature | Directory_ | Y | NULL | NULL | Directory | 1 | UpperCase | NULL | The name of the Directory that can be configured by the UI. A non-null value will enable the browse button. = row
  only A: _Validation: Feature | Display | Y | 0 | 32767 | NULL | NULL | NULL | NULL | Numeric sort order, used to force a specific display ordering. = row
  only A: _Validation: Feature | Feature | N | NULL | NULL | NULL | NULL | Identifier | NULL | Primary key used to identify a particular feature record. = row
  only A: _Validation: Feature | Feature_Parent | Y | NULL | NULL | Feature | 1 | Identifier | NULL | Optional key of a parent record in the same table. If the parent is not selected, then the record will not be installed. Null indicates a root item. = row
  only A: _Validation: Feature | Level | N | 0 | 32767 | NULL | NULL | NULL | NULL | The install level at which record will be initially selected. An install level of 0 will disable an item and prevent its display. = row
  only A: _Validation: Feature | Title | Y | NULL | NULL | NULL | NULL | Text | NULL | Short text identifying a visible feature item. = row
  only A: _Validation: FeatureComponents | Component_ | N | NULL | NULL | Component | 1 | Identifier | NULL | Foreign key into Component table. = row
  only A: _Validation: FeatureComponents | Feature_ | N | NULL | NULL | Feature | 1 | Identifier | NULL | Foreign key into Feature table. = row
  only A: _Validation: File | Attributes | Y | 0 | 32767 | NULL | NULL | NULL | NULL | Integer containing bit flags representing file attributes (with the decimal value of each bit position in parentheses) = row
  only A: _Validation: File | Component_ | N | NULL | NULL | Component | 1 | Identifier | NULL | Foreign key referencing Component that controls the file. = row
  only A: _Validation: File | File | N | NULL | NULL | NULL | NULL | Identifier | NULL | Primary key, non-localized token, must match identifier in cabinet.  For uncompressed files, this field is ignored. = row
  only A: _Validation: File | FileName | N | NULL | NULL | NULL | NULL | Filename | NULL | File name used for installation, may be localized.  This may contain a "short name|long name" pair. = row
  only A: _Validation: File | FileSize | N | 0 | 2147483647 | NULL | NULL | NULL | NULL | Size of file in bytes (long integer). = row
  only A: _Validation: File | Language | Y | NULL | NULL | NULL | NULL | Language | NULL | List of decimal language Ids, comma-separated if more than one. = row
  only A: _Validation: File | Sequence | N | 1 | 2147483647 | NULL | NULL | NULL | NULL | Sequence with respect to the media images; order must track cabinet order. = row
  only A: _Validation: File | Version | Y | NULL | NULL | File | 1 | Version | NULL | Version string for versioned files;  Blank for unversioned files. = row
  only A: _Validation: InstallExecuteSequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL. = row
  only A: _Validation: InstallExecuteSequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse. If the expression syntax is invalid, the engine will terminate, returning iesBadActionData. = row
  only A: _Validation: InstallExecuteSequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed. Leave blank to suppress action. = row
  only A: _Validation: InstallUISequence | Action | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of action to invoke, either in the engine or the handler DLL. = row
  only A: _Validation: InstallUISequence | Condition | Y | NULL | NULL | NULL | NULL | Condition | NULL | Optional expression which skips the action if evaluates to expFalse. If the expression syntax is invalid, the engine will terminate, returning iesBadActionData. = row
  only A: _Validation: InstallUISequence | Sequence | Y | -4 | 32767 | NULL | NULL | NULL | NULL | Number that determines the sort order in which the actions are to be executed. Leave blank to suppress action. = row
  only A: _Validation: LaunchCondition | Condition | N | NULL | NULL | NULL | NULL | Condition | NULL | Expression which must evaluate to TRUE in order for install to commence. = row
  only A: _Validation: LaunchCondition | Description | N | NULL | NULL | NULL | NULL | Formatted | NULL | Localizable text to display when condition fails and install must abort. = row
  only A: _Validation: Media | Cabinet | Y | NULL | NULL | NULL | NULL | Cabinet | NULL | If some or all of the files stored on the media are compressed in a cabinet, the name of that cabinet. = row
  only A: _Validation: Media | DiskId | N | 1 | 32767 | NULL | NULL | NULL | NULL | Primary key, integer to determine sort order for table. = row
  only A: _Validation: Media | DiskPrompt | Y | NULL | NULL | NULL | NULL | Text | NULL | Disk name: the visible text actually printed on the disk.  This will be used to prompt the user when this disk needs to be inserted. = row
  only A: _Validation: Media | LastSequence | N | 0 | 2147483647 | NULL | NULL | NULL | NULL | File sequence number for the last file for this media. = row
  only A: _Validation: Media | Source | Y | NULL | NULL | NULL | NULL | Property | NULL | The property defining the location of the cabinet file. = row
  only A: _Validation: Media | VolumeLabel | Y | NULL | NULL | NULL | NULL | Text | NULL | The label attributed to the volume. = row
  only A: _Validation: MsiFileHash | File_ | N | NULL | NULL | File | 1 | Identifier | NULL | Primary key, foreign key into File table referencing file with this hash = row
  only A: _Validation: MsiFileHash | HashPart1 | N | NULL | NULL | NULL | NULL | NULL | NULL | Size of file in bytes (long integer). = row
  only A: _Validation: MsiFileHash | HashPart2 | N | NULL | NULL | NULL | NULL | NULL | NULL | Size of file in bytes (long integer). = row
  only A: _Validation: MsiFileHash | HashPart3 | N | NULL | NULL | NULL | NULL | NULL | NULL | Size of file in bytes (long integer). = row
  only A: _Validation: MsiFileHash | HashPart4 | N | NULL | NULL | NULL | NULL | NULL | NULL | Size of file in bytes (long integer). = row
  only A: _Validation: MsiFileHash | Options | N | 0 | 32767 | NULL | NULL | NULL | NULL | Various options and attributes for this hash. = row
  only A: _Validation: Property | Property | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of property, uppercase if settable by launcher or loader. = row
  only A: _Validation: Property | Value | N | NULL | NULL | NULL | NULL | Text | NULL | String value for property.  Never null or empty. = row
  only A: _Validation: Upgrade | ActionProperty | N | NULL | NULL | NULL | NULL | UpperCase | NULL | The property to set when a product in this set is found. = row
  only A: _Validation: Upgrade | Attributes | N | 0 | 2147483647 | NULL | NULL | NULL | NULL | The attributes of this product set. = row
  only A: _Validation: Upgrade | Language | Y | NULL | NULL | NULL | NULL | Language | NULL | A comma-separated list of languages for either products in this set or products not in this set. = row
  only A: _Validation: Upgrade | Remove | Y | NULL | NULL | NULL | NULL | Formatted | NULL | The list of features to remove when uninstalling a product from this set.  The default is "ALL". = row
  only A: _Validation: Upgrade | UpgradeCode | N | NULL | NULL | NULL | NULL | Guid | NULL | The UpgradeCode GUID belonging to the products in this set. = row
  only A: _Validation: Upgrade | VersionMax | Y | NULL | NULL | NULL | NULL | Text | NULL | The maximum ProductVersion of the products in this set.  The set may or may not include products with this particular version. = row
  only A: _Validation: Upgrade | VersionMin | Y | NULL | NULL | NULL | NULL | Text | NULL | The minimum ProductVersion of the products in this set.  The set may or may not include products with this particular version. = row
  only A: _Validation: _SummaryInformation | PropertyId | N | NULL | NULL | NULL | NULL | NULL | NULL | NULL = row
  only A: _Validation: _SummaryInformation | Value | N | NULL | NULL | NULL | NULL | NULL | NULL | NULL = row
  only A: _Validation: _Validation | Category | Y | NULL | NULL | NULL | NULL | NULL | Text;Formatted;Template;Condition;Guid;Path;Version;Language;Identifier;Binary;UpperCase;LowerCase;Filename;Paths;AnyPath;WildCardFilename;RegPath;CustomSource;Property;Cabinet;Shortcut;FormattedSDDLText;Integer;DoubleInteger;TimeDate;DefaultDir | String category = row
  only A: _Validation: _Validation | Column | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of column = row
  only A: _Validation: _Validation | Description | Y | NULL | NULL | NULL | NULL | Text | NULL | Description of column = row
  only A: _Validation: _Validation | KeyColumn | Y | 1 | 32 | NULL | NULL | NULL | NULL | Column to which foreign key connects = row
  only A: _Validation: _Validation | KeyTable | Y | NULL | NULL | NULL | NULL | Identifier | NULL | For foreign key, Name of table to which data must link = row
  only A: _Validation: _Validation | MaxValue | Y | -2147483647 | 2147483647 | NULL | NULL | NULL | NULL | Maximum value allowed = row
  only A: _Validation: _Validation | MinValue | Y | -2147483647 | 2147483647 | NULL | NULL | NULL | NULL | Minimum value allowed = row
  only A: _Validation: _Validation | Nullable | N | NULL | NULL | NULL | NULL | NULL | Y;N | Whether the column is nullable = row
  only A: _Validation: _Validation | Set | Y | NULL | NULL | NULL | NULL | Text | NULL | Set of values that are permitted = row
  only A: _Validation: _Validation | Table | N | NULL | NULL | NULL | NULL | Identifier | NULL | Name of table = row
