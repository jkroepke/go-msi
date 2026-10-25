package config

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}

	return m
}

// standardFolders are the Windows Installer folder properties a top-level
// directory may name.
var standardFolders = set(
	"TARGETDIR", "AdminToolsFolder", "AppDataFolder", "CommonAppDataFolder",
	"CommonFiles64Folder", "CommonFilesFolder", "DesktopFolder", "FavoritesFolder",
	"FontsFolder", "LocalAppDataFolder", "MyPicturesFolder", "NetHoodFolder",
	"PersonalFolder", "PrintHoodFolder", "ProgramFiles64Folder", "ProgramFilesFolder",
	"ProgramMenuFolder", "RecentFolder", "SendToFolder", "StartMenuFolder",
	"StartupFolder", "System16Folder", "System64Folder", "SystemFolder",
	"TempFolder", "TemplateFolder", "WindowsFolder", "WindowsVolume",
)

// folders32 are the 32-bit locations on 64-bit Windows (ICE80).
var folders32 = set("ProgramFilesFolder", "CommonFilesFolder", "SystemFolder")

// perMachineFolders cannot hold resources of a per-user package (ICE105).
var perMachineFolders = set(
	"CommonAppDataFolder", "CommonFiles64Folder", "CommonFilesFolder",
	"ProgramFiles64Folder", "ProgramFilesFolder", "System16Folder",
	"System64Folder", "SystemFolder", "WindowsFolder", "FontsFolder",
)

// standardActions may not be used as custom action ids (ICE93) and are valid
// placement anchors.
var standardActions = set(
	"ADMIN", "ADVERTISE", "AllocateRegistrySpace", "AppSearch", "BindImage",
	"CCPSearch", "CostFinalize", "CostInitialize", "CreateFolders", "CreateShortcuts",
	"DeleteServices", "DisableRollback", "DuplicateFiles", "ExecuteAction", "FileCost",
	"FindRelatedProducts", "ForceReboot", "INSTALL", "InstallAdminPackage",
	"InstallExecute", "InstallExecuteAgain", "InstallFiles", "InstallFinalize",
	"InstallInitialize", "InstallODBC", "InstallServices", "InstallSFPCatalogFile",
	"InstallValidate", "IsolateComponents", "LaunchConditions", "MigrateFeatureStates",
	"MoveFiles", "MsiConfigureServices", "MsiPublishAssemblies", "MsiUnpublishAssemblies",
	"PatchFiles", "ProcessComponents", "PublishComponents", "PublishFeatures",
	"PublishProduct", "RegisterClassInfo", "RegisterComPlus", "RegisterExtensionInfo",
	"RegisterFonts", "RegisterMIMEInfo", "RegisterProduct", "RegisterProgIdInfo",
	"RegisterTypeLibraries", "RegisterUser", "RemoveDuplicateFiles",
	"RemoveEnvironmentStrings", "RemoveExistingProducts", "RemoveFiles", "RemoveFolders",
	"RemoveIniValues", "RemoveODBC", "RemoveRegistryValues", "RemoveShortcuts",
	"ResolveSource", "RMCCPSearch", "ScheduleReboot", "SelfRegModules",
	"SelfUnregModules", "SetODBCFolders", "StartServices", "StopServices",
	"UnpublishComponents", "UnpublishFeatures", "UnregisterClassInfo",
	"UnregisterComPlus", "UnregisterExtensionInfo", "UnregisterFonts",
	"UnregisterMIMEInfo", "UnregisterProgIdInfo", "UnregisterTypeLibraries",
	"ValidateProductID", "WriteEnvironmentStrings", "WriteIniValues",
	"WriteRegistryValues",
)

// builderProperties are written by the builder or set by the engine, so a
// package cannot define them in properties (ICE87 covers the request ones).
var builderProperties = set(
	"ProductCode", "ProductName", "ProductVersion", "ProductLanguage", "Manufacturer",
	"UpgradeCode", "ALLUSERS", "SecureCustomProperties", "MsiHiddenProperties",
	"ADDLOCAL", "ADDSOURCE", "ADDDEFAULT", "ADVERTISE", "COMPADDLOCAL",
	"COMPADDSOURCE", "COMPADDDEFAULT", "FILEADDLOCAL", "FILEADDSOURCE",
	"FILEADDDEFAULT", "REINSTALL", "REMOVE", "ARPPRODUCTICON", "ARPNOREPAIR",
	"ARPNOMODIFY", "ARPNOREMOVE", "ARPHELPLINK", "ARPURLINFOABOUT", "ARPURLUPDATEINFO",
	"ARPCOMMENTS", "ARPCONTACT", "ARPSIZE", "ARPINSTALLLOCATION",
)

// standardTables are written by the builder and cannot be redefined in tables.
var standardTables = set(
	"_Columns", "_Storages", "_Streams", "_StringData", "_StringPool", "_Tables",
	"_Validation", "ActionText", "AdminExecuteSequence", "AdminUISequence",
	"AdvtExecuteSequence", "AdvtUISequence", "AppSearch", "Binary", "BBControl",
	"Billboard", "CheckBox", "ComboBox", "Component", "Condition", "Control",
	"ControlCondition", "ControlEvent", "CreateFolder", "CustomAction", "Dialog",
	"Directory", "DrLocator", "Environment", "Error", "EventMapping", "Feature",
	"FeatureComponents", "File", "Icon", "InstallExecuteSequence", "InstallUISequence",
	"LaunchCondition", "ListBox", "ListView", "Media", "MsiFileHash",
	"MsiServiceConfig", "MsiServiceConfigFailureActions", "Property", "RadioButton",
	"RegLocator", "Registry", "RemoveFile", "RemoveRegistry", "ServiceControl",
	"ServiceInstall", "Shortcut", "Signature", "TextStyle", "UIText", "Upgrade",
)

// sequenceTables maps the schedule names to the sequence tables.
var sequenceTables = map[string]string{
	"execute":      "InstallExecuteSequence",
	"ui":           "InstallUISequence",
	"adminExecute": "AdminExecuteSequence",
	"adminUI":      "AdminUISequence",
}

// peExtensions mark files that are expected to carry a version resource.
var peExtensions = set(".exe", ".dll", ".sys", ".ocx", ".cpl", ".drv", ".scr", ".efi", ".mui")
