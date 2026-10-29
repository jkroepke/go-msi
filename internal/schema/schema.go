// Package schema defines the standard Windows Installer tables that msi-builder
// writes: their columns in .idt notation and the _Validation metadata of
// every column (docs/spec/MSISPEC.md section 6). The definitions follow the
// published table reference and the toolchain conventions the specification
// records (File.Sequence and Media.LastSequence are i4, the 26-name
// Category set).
package schema

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/internal/msidb"
)

// ColumnDef is a column with its _Validation row.
type ColumnDef struct {
	MinValue    *int
	MaxValue    *int
	Name        string
	Category    string
	KeyTable    string
	Set         string
	Description string
	Type        msidb.Type
	KeyColumn   int
}

// TableDef is a standard table.
type TableDef struct {
	Name    string
	Columns []ColumnDef
}

// Table returns an empty msidb.Table with the definition's columns.
func (d *TableDef) Table() *msidb.Table {
	t := &msidb.Table{Name: d.Name}
	for _, c := range d.Columns {
		t.Columns = append(t.Columns, msidb.Column{Name: c.Name, Type: c.Type})
	}

	return t
}

// Categories is the _Validation.Category set the toolchains write.
const Categories = "Text;Formatted;Template;Condition;Guid;Path;Version;Language;Identifier;Binary;UpperCase;LowerCase;Filename;" +
	"Paths;AnyPath;WildCardFilename;RegPath;CustomSource;Property;Cabinet;Shortcut;DefaultDir;FormattedSDDLText;Integer;DoubleInteger;TimeDate"

const controlRef = "Control_|s50|K|Identifier|Control|2||||A foreign key to the Control table, name of the control."

// Each line: Name|Type|K (key) or empty|Category|KeyTable|KeyColumn|Min|Max|Set|Description.
var defs = map[string][]string{ //nolint:gochecknoglobals // lookup table
	"_Validation": {
		"Table|s32|K|Identifier||||||Name of table",
		"Column|s32|K|Identifier||||||Name of column",
		"Nullable|s4|||||||Y;N|Whether the column is nullable",
		"MinValue|I4|||||-2147483647|2147483647||Minimum value allowed",
		"MaxValue|I4|||||-2147483647|2147483647||Maximum value allowed",
		"KeyTable|S255||Identifier||||||For foreign key, Name of table to which data must link",
		"KeyColumn|I2|||||1|32||Column to which foreign key connects",
		"Category|S32|||||||" + Categories + "|String category",
		"Set|S255||Text||||||Set of values that are permitted",
		"Description|S255||Text||||||Description of column",
	},
	"ActionText": {
		"Action|s72|K|Identifier||||||Name of action to be described.",
		"Description|L64||Text||||||Localized description displayed in progress dialog and log when action is executing.",
		"Template|L128||Template||||||Optional localized format template used to format action data records for display during action execution.",
	},
	"AdminExecuteSequence":   sequenceColumns,
	"AdminUISequence":        sequenceColumns,
	"AdvtExecuteSequence":    sequenceColumns,
	"InstallExecuteSequence": sequenceColumns,
	"InstallUISequence":      sequenceColumns,
	"Binary": {
		"Name|s72|K|Identifier||||||Unique key identifying the binary data.",
		"Data|v0||Binary||||||The unformatted binary data.",
	},
	"CheckBox": {
		"Property|s72|K|Identifier||||||A named property to be tied to the item.",
		"Value|S64||Formatted||||||The value string associated with the item.",
	},
	"Control": {
		"Dialog_|s72|K|Identifier|Dialog|1||||External key to the Dialog table, name of the dialog.",
		"Control|s50|K|Identifier||||||Name of the control. This name must be unique within a dialog, but can repeat on different dialogs.",
		"Type|s20||Identifier||||||The type of the control.",
		"X|i2|||||0|32767||Horizontal coordinate of the upper left corner of the bounding rectangle of the control.",
		"Y|i2|||||0|32767||Vertical coordinate of the upper left corner of the bounding rectangle of the control.",
		"Width|i2|||||0|32767||Width of the bounding rectangle of the control.",
		"Height|i2|||||0|32767||Height of the bounding rectangle of the control.",
		"Attributes|I4|||||0|2147483647||A 32-bit word that specifies the attribute flags to be applied to this control.",
		"Property|S50||Identifier||||||The name of a defined property to be linked to this control.",
		"Text|L0||Formatted||||||A string used to set the initial text contained within a control (if appropriate).",
		"Control_Next|S50||Identifier|Control|2||||The name of an other control on the same dialog. This link defines the tab order of the controls.",
		"Help|L50||Text||||||The help strings used with the button. The text is optional.",
	},
	"ControlCondition": {
		"Dialog_|s72|K|Identifier|Dialog|1||||A foreign key to the Dialog table, name of the dialog.",
		controlRef,
		"Action|s50|K|Text|||||Default;Disable;Enable;Hide;Show|The desired action to be taken on the specified control.",
		"Condition|s255|K|Condition||||||A standard conditional statement that specifies under which conditions the action should be triggered.",
	},
	"ControlEvent": {
		"Dialog_|s72|K|Identifier|Dialog|1||||A foreign key to the Dialog table, name of the dialog.",
		controlRef,
		"Event|s50|K|Formatted||||||An identifier that specifies the type of the event that should take place when the user interacts with control " +
			"specified by the first two entries.",
		"Argument|s255|K|Formatted||||||A value to be used as a modifier when triggering a particular event.",
		"Condition|S255|K|Condition||||||A standard conditional statement that specifies under which conditions an event should be triggered.",
		"Ordering|I2|||||0|2147483647||An integer used to order several events tied to the same control. Can be left blank.",
	},
	"Dialog": {
		"Dialog|s72|K|Identifier||||||Name of the dialog.",
		"HCentering|i2|||||0|100||Horizontal position of the dialog on a 0-100 scale. 0 means left end, 100 means right end of the screen, 50 center.",
		"VCentering|i2|||||0|100||Vertical position of the dialog on a 0-100 scale. 0 means top end, 100 means bottom end of the screen, 50 center.",
		"Width|i2|||||0|32767||Width of the bounding rectangle of the dialog.",
		"Height|i2|||||0|32767||Height of the bounding rectangle of the dialog.",
		"Attributes|I4|||||0|2147483647||A 32-bit word that specifies the attribute flags to be applied to this dialog.",
		"Title|L128||Formatted||||||A text string specifying the title to be displayed in the title bar of the dialog's window.",
		"Control_First|s50||Identifier|Control|2||||Defines the control that has the focus when the dialog is created.",
		"Control_Default|S50||Identifier|Control|2||||Defines the default control. Hitting return is equivalent to pushing this button.",
		"Control_Cancel|S50||Identifier|Control|2||||Defines the cancel control. Hitting escape or clicking on the close icon on the dialog is equivalent " +
			"to pushing this button.",
	},
	"Component": {
		"Component|s72|K|Identifier||||||Primary key used to identify a particular component record.",
		"ComponentId|S38||Guid||||||A string GUID unique to this component, version, and language.",
		"Directory_|s72||Identifier|Directory|1||||Required key of a Directory table record.",
		"Attributes|i2||||||||Remote execution option, one of irsEnum",
		"Condition|S255||Condition||||||A conditional statement that will disable this component if the specified condition evaluates to the 'True' state.",
		"KeyPath|S72||Identifier|File;Registry;ODBCDataSource|1||||Either the primary key into the File table, Registry table, or ODBCDataSource table.",
	},
	"CreateFolder": {
		"Directory_|s72|K|Identifier|Directory|1||||Primary key, could be foreign key into the Directory table.",
		"Component_|s72|K|Identifier|Component|1||||Foreign key into the Component table.",
	},
	"CustomAction": {
		"Action|s72|K|Identifier||||||Primary key, name of action, normally appears in sequence table unless private use.",
		"Type|i2|||||1|32767||The numeric custom action type, consisting of source location, code type, entry, option flags.",
		"Source|S72||CustomSource||||||The table reference of the source of the code.",
		"Target|S255||Formatted||||||Execution parameter, depends on the type of custom action",
		"ExtendedType|I4|||||0|2147483647||A numeric custom action type that extends code type or option flags of the Type column.",
	},
	"Directory": {
		"Directory|s72|K|Identifier||||||Unique identifier for directory entry, primary key.",
		"Directory_Parent|S72||Identifier|Directory|1||||Reference to the entry in this table specifying the default parent directory.",
		"DefaultDir|l255||DefaultDir||||||The default sub-path under parent's path.",
	},
	"Environment": {
		"Environment|s72|K|Identifier||||||Unique identifier for the environmental variable setting",
		"Name|l255||Text||||||The name of the environmental value.",
		"Value|L255||Formatted||||||The value to set in the environmental settings.",
		"Component_|s72||Identifier|Component|1||||Foreign key into the Component table referencing component that controls the installing of the.",
	},
	"EventMapping": {
		"Dialog_|s72|K|Identifier|Dialog|1||||A foreign key to the Dialog table, name of the Dialog.",
		controlRef,
		"Event|s50|K|Identifier||||||An identifier that specifies the type of the event that the control subscribes to.",
		"Attribute|s50||Identifier||||||The name of the control attribute, that is set when this event is received.",
	},
	"Error": {
		"Error|i2|K||||0|32767||Integer error number, obtained from header file IError(...) macros.",
		"Message|L0||Template||||||Error formatting template, obtained from user ed. or localizers.",
	},
	"Feature": {
		"Feature|s38|K|Identifier||||||Primary key used to identify a particular feature record.",
		"Feature_Parent|S38||Identifier|Feature|1||||Optional key of a parent record in the same table.",
		"Title|L64||Text||||||Short text identifying a visible feature item.",
		"Description|L255||Text||||||Longer descriptive text describing a visible feature item.",
		"Display|I2|||||0|32767||Numeric sort order, used to force a specific display ordering.",
		"Level|i2|||||0|32767||The install level at which record will be initially selected.",
		"Directory_|S72||UpperCase|Directory|1||||The name of the Directory that can be configured by the UI.",
		"Attributes|i2|||||||0;1;2;4;5;6;8;9;10;16;17;18;20;21;22;24;25;26;32;33;34;36;37;38;48;49;50;52;53;54|Feature attributes",
	},
	"FeatureComponents": {
		"Feature_|s38|K|Identifier|Feature|1||||Foreign key into Feature table.",
		"Component_|s72|K|Identifier|Component|1||||Foreign key into Component table.",
	},
	"File": {
		"File|s72|K|Identifier||||||Primary key, non-localized token, must match identifier in cabinet.",
		"Component_|s72||Identifier|Component|1||||Foreign key referencing Component that controls the file.",
		"FileName|l255||Filename||||||File name used for installation, may be localized.",
		"FileSize|i4|||||0|2147483647||Size of file in bytes (long integer).",
		"Version|S72||Version|File|1||||Version string for versioned files; Blank for unversioned files.",
		"Language|S20||Language||||||List of decimal language Ids, comma-separated if more than one.",
		"Attributes|I2|||||0|32767||Integer containing bit flags representing file attributes",
		"Sequence|i4|||||1|2147483647||Sequence with respect to the media images; order must track cabinet order.",
	},
	"Icon": {
		"Name|s72|K|Identifier||||||Primary key. Name of the icon file.",
		"Data|v0||Binary||||||Binary stream. The binary icon data in PE (.DLL or .EXE) or icon (.ICO) format.",
	},
	"LaunchCondition": {
		"Condition|s255|K|Condition||||||Expression which must evaluate to TRUE in order for install to commence.",
		"Description|l255||Formatted||||||Localizable text to display when condition fails and install must abort.",
	},
	"ListBox": {
		"Property|s72|K|Identifier||||||A named property to be tied to this item. All the items tied to the same property become part of the same listbox.",
		"Order|i2|K||||1|32767||A positive integer used to determine the ordering of the items within one list..The integers do not have to be consecutive.",
		"Value|s64||Formatted||||||The value string associated with this item. Selecting the line will set the associated property to this value.",
		"Text|L64||Text||||||The visible text to be assigned to the item. Optional. If this entry or the entire column is missing, the text is the same " +
			"as the value.",
	},
	"Media": {
		"DiskId|i2|K||||1|32767||Primary key, integer to determine sort order for table.",
		"LastSequence|i4|||||0|2147483647||File sequence number for the last file for this media.",
		"DiskPrompt|L64||Text||||||Disk name: the visible text actually printed on the disk.",
		"Cabinet|S255||Cabinet||||||If some or all of the files stored on the media are compressed in a cabinet, the name of that cabinet.",
		"VolumeLabel|S32||Text||||||The label attributed to the volume.",
		"Source|S72||Property||||||The property defining the location of the cabinet file.",
	},
	"MsiDigitalCertificate": {
		"DigitalCertificate|s72|K|Identifier||||||Primary key, the name of the certificate.",
		"CertData|v0||Binary||||||The DER-encoded certificate.",
	},
	"MsiDigitalSignature": {
		"Table|s32|K|Identifier|||||Media|The table of the signed object; only Media is checked.",
		"SignObject|s72|K|Text||||||The primary key of the signed object in Table, a Media.DiskId.",
		"DigitalCertificate_|s72||Identifier|MsiDigitalCertificate|1||||Foreign key into MsiDigitalCertificate, the certificate the object must be signed with.",
		"Hash|V0||Binary||||||The signer's signature value, as MsiGetFileSignatureInformation reports it; null checks the certificate only.",
	},
	"MsiFileHash": {
		"File_|s72|K|Identifier|File|1||||Primary key, foreign key into File table referencing file with this hash",
		"Options|i2|||||0|32767||Various options and attributes for this hash.",
		"HashPart1|i4||||||||Size of file in bytes (long integer).",
		"HashPart2|i4||||||||Size of file in bytes (long integer).",
		"HashPart3|i4||||||||Size of file in bytes (long integer).",
		"HashPart4|i4||||||||Size of file in bytes (long integer).",
	},
	"MsiLockPermissionsEx": {
		"MsiLockPermissionsEx|s72|K|Identifier||||||Primary key, non-localized token.",
		"LockObject|s72||Identifier||||||Foreign key into Registry, File, CreateFolder, or ServiceInstall table.",
		"Table|s32||Identifier|||||CreateFolder;File;Registry;ServiceInstall|Reference to another table name " +
			"(only Registry, File, CreateFolder, and ServiceInstall).",
		"SDDLText|s0||FormattedSDDLText||||||String to indicate permissions to be applied to the LockObject.",
		"Condition|S255||Condition||||||Expression which must evaluate to TRUE in order for this row to be applied.",
	},
	"MsiShortcutProperty": {
		"MsiShortcutProperty|s72|K|Identifier||||||Primary key, non-localized token.",
		"Shortcut_|s72||Identifier|Shortcut|1||||Foreign key into the Shortcut table.",
		"PropertyKey|s0||Formatted||||||Canonical name of a shell property, such as System.AppUserModel.ID.",
		"PropVariantValue|s0||Formatted||||||The value of the property.",
	},
	"MsiServiceConfig": {
		"MsiServiceConfig|s72|K|Identifier||||||Primary key, non-localized token.",
		"Name|s255||Formatted||||||Name of a service.",
		"Event|i2|||||0|7||Bit field: Install (1), Uninstall (2), Reinstall (4).",
		"ConfigType|i4|||||1|7||Service configuration option.",
		"Argument|L0||Formatted||||||Argument(s) for service configuration.",
		"Component_|s72||Identifier|Component|1||||Required foreign key into the Component Table.",
	},
	"Property": {
		"Property|s72|K|Identifier||||||Name of property, uppercase if settable by launcher or loader.",
		"Value|l0||Text||||||String value for property.",
	},
	"RadioButton": {
		"Property|s72|K|Identifier||||||A named property to be tied to this radio button. All the buttons tied to the same property become part of the same group.",
		"Order|i2|K||||1|32767||A positive integer used to determine the ordering of the items within one list..The integers do not have to be consecutive.",
		"Value|s64||Formatted||||||The value string associated with this button. Selecting the button will set the associated property to this value.",
		"X|i2|||||0|32767||The horizontal coordinate of the upper left corner of the bounding rectangle of the radio button.",
		"Y|i2|||||0|32767||The vertical coordinate of the upper left corner of the bounding rectangle of the radio button.",
		"Width|i2|||||0|32767||The width of the button.",
		"Height|i2|||||0|32767||The height of the button.",
		"Text|L64||Text||||||The visible title to be assigned to the radio button.",
		"Help|L50||Text||||||The help strings used with the button. The text is optional.",
	},
	"AppSearch": {
		"Property|s72|K|Identifier||||||The property set to the path a search finds.",
		"Signature_|s72|K|Identifier||||||The search: a key into Signature and the locator tables.",
	},
	"CompLocator": {
		"Signature_|s72|K|Identifier||||||The search this component location belongs to.",
		"ComponentId|s38||Guid||||||The component code whose key path is searched.",
		"Type|I2|||||0|1||0: the key path is a directory; 1: a file.",
	},
	"DrLocator": {
		"Signature_|s72|K|Identifier||||||The search this folder location belongs to.",
		"Parent|S72|K|Identifier||||||The search that finds the folder Path is relative to.",
		"Path|S255|K|AnyPath||||||A full path, or one relative to Parent; may name properties in brackets.",
		"Depth|I2|||||0|32767||How many levels below Path are searched.",
	},
	"RegLocator": {
		"Signature_|s72|K|Identifier||||||The search this registry location belongs to.",
		"Root|i2|||||0|3||The predefined registry root.",
		"Key|s255||RegPath||||||The registry key.",
		"Name|S255||Formatted||||||The registry value name; empty for the default value.",
		"Type|I2|||||0|18||What the value names (file, directory or raw), plus 16 for the 64-bit view.",
	},
	"Signature": {
		"Signature|s72|K|Identifier||||||The file signature a search compares.",
		"FileName|s255||Filename||||||The file name.",
		"MinVersion|S20||Text||||||The lowest version.",
		"MaxVersion|S20||Text||||||The highest version.",
		"MinSize|I4|||||0|2147483647||The smallest size.",
		"MaxSize|I4|||||0|2147483647||The largest size.",
		"MinDate|I4|||||0|2147483647||The earliest modification time.",
		"MaxDate|I4|||||0|2147483647||The latest modification time.",
		"Languages|S255||Language||||||The languages.",
	},
	"Registry": {
		"Registry|s72|K|Identifier||||||Primary key, non-localized token.",
		"Root|i2|||||-1|3||The predefined root key for the registry value, one of rrkEnum.",
		"Key|l255||RegPath||||||The key for the registry value.",
		"Name|L255||Formatted||||||The registry value name.",
		"Value|L0||Formatted||||||The registry value.",
		"Component_|s72||Identifier|Component|1||||Foreign key into the Component table referencing component that controls the installing of the registry value.",
	},
	"IniFile": {
		"IniFile|s72|K|Identifier||||||Primary key, non-localized token.",
		"FileName|l255||Filename||||||The .ini file name in its directory.",
		"DirProperty|S72||Identifier||||||Name of a property whose value resolves to the folder of the .ini file.",
		"Section|l96||Formatted||||||The .ini file Section.",
		"Key|l128||Formatted||||||The .ini file key below Section.",
		"Value|l255||Formatted||||||The value to be written.",
		"Action|i2|||||||0;1;3|The type of modification to be made, one of iifEnum",
		"Component_|s72||Identifier|Component|1||||Foreign key into the Component table referencing component that controls the installing of the .ini value.",
	},
	"RemoveFile": {
		"FileKey|s72|K|Identifier||||||Primary key used to identify a particular file entry",
		"Component_|s72||Identifier|Component|1||||Foreign key referencing Component that controls the file to be removed.",
		"FileName|L255||WildCardFilename||||||Name of the file to be removed.",
		"DirProperty|s72||Identifier||||||Name of a property whose value is assumed to resolve to the full pathname to the folder of the file to be removed.",
		"InstallMode|i2|||||||1;2;3|Installation option, one of iimEnum.",
	},
	"RemoveRegistry": {
		"RemoveRegistry|s72|K|Identifier||||||Primary key, non-localized token.",
		"Root|i2|||||-1|3||The predefined root key for the registry value, one of rrkEnum",
		"Key|l255||RegPath||||||The key for the registry value.",
		"Name|L255||Formatted||||||The registry value name.",
		"Component_|s72||Identifier|Component|1||||Foreign key into the Component table referencing component that controls the deletion of the registry value.",
	},
	"ServiceControl": {
		"ServiceControl|s72|K|Identifier||||||Primary key, non-localized token.",
		"Name|l255||Formatted||||||Name of a service. /, \\, comma and space are invalid",
		"Event|i2|||||0|187||Bit field: Install: 0x1 = Start, 0x2 = Stop, 0x8 = Delete, Uninstall: 0x10 = Start, 0x20 = Stop, 0x80 = Delete",
		"Arguments|L255||Formatted||||||Arguments for the service. Separate by [~].",
		"Wait|I2|||||0|1||Boolean for whether to wait for the service to fully start",
		"Component_|s72||Identifier|Component|1||||Required foreign key into the Component Table that controls the startup of the service",
	},
	"ServiceInstall": {
		"ServiceInstall|s72|K|Identifier||||||Primary key, non-localized token.",
		"Name|s255||Formatted||||||Internal Name of the Service",
		"DisplayName|L255||Formatted||||||External Name of the Service",
		"ServiceType|i4|||||-2147483647|2147483647||Type of the service",
		"StartType|i4|||||0|4||Type of the service",
		"ErrorControl|i4|||||-2147483647|2147483647||Severity of error if service fails to start",
		"LoadOrderGroup|S255||Formatted||||||LoadOrderGroup",
		"Dependencies|S255||Formatted||||||Other services this depends on to start. Separate by [~], and end with [~][~]",
		"StartName|S255||Formatted||||||User or object name to run service as",
		"Password|S255||Formatted||||||password to run service with. (with StartName)",
		"Arguments|S255||Formatted||||||Arguments for the service. Separate by [~].",
		"Component_|s72||Identifier|Component|1||||Required foreign key into the Component Table that controls the startup of the service",
		"Description|L255||Text||||||Description of service.",
	},
	"Shortcut": {
		"Shortcut|s72|K|Identifier||||||Primary key, non-localized token.",
		"Directory_|s72||Identifier|Directory|1||||Foreign key into the Directory table denoting the directory where the shortcut file is created.",
		"Name|l128||Filename||||||The name of the shortcut to be created.",
		"Component_|s72||Identifier|Component|1||||Foreign key into the Component table denoting the component whose selection gates the shortcut creation/deletion.",
		"Target|s72||Shortcut||||||The shortcut target: a Feature key for an advertised shortcut, otherwise a formatted path.",
		"Arguments|S255||Formatted||||||The command-line arguments for the shortcut.",
		"Description|L255||Text||||||The description for the shortcut.",
		"Hotkey|I2|||||0|32767||The hotkey for the shortcut: the virtual-key code in the low byte, the modifier flags in the high byte.",
		"Icon_|S72||Identifier|Icon|1||||Foreign key into the Icon table.",
		"IconIndex|I2|||||-32767|32767||The icon index for the shortcut.",
		"ShowCmd|I2|||||||1;3;7|The show command for the application window.",
		"WkDir|S72||Identifier||||||Name of the property that has the path of the working directory for the shortcut.",
		"DisplayResourceDLL|S255||Formatted||||||The path to the language-neutral file that holds the display name resource.",
		"DisplayResourceId|I2|||||0|32767||The display name index for the shortcut.",
		"DescriptionResourceDLL|S255||Formatted||||||The path to the language-neutral file that holds the description resource.",
		"DescriptionResourceId|I2|||||0|32767||The description index for the shortcut.",
	},
	"TextStyle": {
		"TextStyle|s72|K|Identifier||||||Name of the style. The primary key of this table. This name is embedded in the texts to indicate a style change.",
		"FaceName|s32||Text||||||A string indicating the name of the font used. Required. The string must be at most 31 characters long.",
		"Size|i2|||||0|32767||The size of the font used. This size is given in our units (1/12 of the system font height). Assuming that the system font " +
			"is set to 12 point size, this is equivalent to the point size.",
		"Color|I4|||||0|16777215||A long integer indicating the color of the string in the RGB format (Red, Green, Blue each 0-255, RGB = R + 256*G + 256^2*B).",
		"StyleBits|I2|||||0|15||A combination of style bits.",
	},
	"UIText": {
		"Key|s72|K|Identifier||||||A unique key that identifies the particular string.",
		"Text|L255||Text||||||The localized version of the string.",
	},
	"Upgrade": {
		"UpgradeCode|s38|K|Guid||||||The UpgradeCode GUID belonging to the products in this set.",
		"VersionMin|S20|K|Text||||||The minimum ProductVersion of the products in this set. The set may or may not include products with this particular version.",
		"VersionMax|S20|K|Text||||||The maximum ProductVersion of the products in this set. The set may or may not include products with this particular version.",
		"Language|S255|K|Language||||||A comma-separated list of languages for either products in this set or products not in this set.",
		"Attributes|i4|K||||0|2147483647||The attributes of this product set.",
		"Remove|S255||Formatted||||||The list of features to remove when uninstalling a product from this set. The default is \"ALL\".",
		"ActionProperty|s72||UpperCase||||||The property to set when a product in this set is found.",
	},
}

var sequenceColumns = []string{ //nolint:gochecknoglobals // lookup table
	"Action|s72|K|Identifier||||||Name of action to invoke, either in the engine or the handler DLL.",
	"Condition|S255||Condition||||||Optional expression which skips the action if evaluates to expFalse.",
	"Sequence|I2|||||-4|32767||Number that determines the sort order in which the actions are to be executed.  Leave blank to suppress action.",
}

var tables = parseTables() //nolint:gochecknoglobals // lookup table

// parseTables turns the DSL lines into table definitions.
func parseTables() map[string]*TableDef {
	out := make(map[string]*TableDef, len(defs))

	for name, lines := range defs {
		def := &TableDef{Name: name}

		for _, line := range lines {
			fields := strings.Split(line, "|")
			if len(fields) != 10 {
				panic(fmt.Sprintf("schema: %s: malformed column %q", name, line))
			}

			col := ColumnDef{Name: fields[0], Type: msidb.MustType(fields[1]), Category: fields[3], KeyTable: fields[4], Set: fields[8], Description: fields[9]}
			col.Type.Key = fields[2] == "K"

			if fields[5] != "" {
				col.KeyColumn, _ = strconv.Atoi(fields[5])
			}

			col.MinValue = optInt(fields[6])
			col.MaxValue = optInt(fields[7])
			def.Columns = append(def.Columns, col)
		}

		out[name] = def
	}

	return out
}

func optInt(s string) *int {
	if s == "" {
		return nil
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}

	return &n
}

// Lookup returns the definition of a standard table.
func Lookup(name string) (*TableDef, bool) {
	d, ok := tables[name]

	return d, ok
}

// New returns an empty table with the standard columns; it panics for an
// unknown table name.
func New(name string) *msidb.Table {
	d, ok := tables[name]
	if !ok {
		panic("schema: unknown table " + name)
	}

	return d.Table()
}

// Names lists the standard tables in sorted order.
func Names() []string {
	out := make([]string, 0, len(tables))
	for n := range tables {
		out = append(out, n)
	}

	slices.Sort(out)

	return out
}

// Validation builds the _Validation table for the given tables: one row per
// column of every table, _Validation itself included, plus the two
// _SummaryInformation rows the toolchains write. Custom tables (not in this
// package) get rows derived from their column types.
func Validation(present []*msidb.Table) *msidb.Table { //nolint:cyclop // one pass over the format; splitting would scatter it
	val := New("_Validation")

	names := make(map[string]*msidb.Table)
	for _, t := range present {
		names[t.Name] = t
	}

	names["_Validation"] = val

	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}

	slices.Sort(sorted)

	for _, name := range sorted {
		tbl := names[name]

		if def, ok := tables[name]; ok {
			for _, c := range def.Columns {
				val.Add(name, c.Name, nullable(c.Type), optCell(c.MinValue), optCell(c.MaxValue), orNil(c.KeyTable), keyCol(c.KeyColumn),
					orNil(c.Category), orNil(c.Set), orNil(c.Description))
			}

			continue
		}

		for _, col := range tbl.Columns {
			var category any

			var minV, maxV any

			switch col.Type.Kind {
			case msidb.KindString:
				category = "Text"
			case msidb.KindBinary:
				category = "Binary"
			case msidb.KindInt16:
				minV, maxV = msidb.MinInt16, msidb.MaxInt16
			case msidb.KindInt32:
				minV, maxV = msidb.MinInt32, msidb.MaxInt32
			}

			val.Add(name, col.Name, nullable(col.Type), minV, maxV, nil, nil, category, nil, nil)
		}
	}

	val.Add("_SummaryInformation", "PropertyId", "N", nil, nil, nil, nil, nil, nil, nil)
	val.Add("_SummaryInformation", "Value", "N", nil, nil, nil, nil, "Text", nil, nil)

	return val
}

func nullable(t msidb.Type) string {
	if t.Nullable {
		return "Y"
	}

	return "N"
}

func optCell(p *int) any {
	if p == nil {
		return nil
	}

	return *p
}

func orNil(s string) any {
	if s == "" {
		return nil
	}

	return s
}

func keyCol(n int) any {
	if n == 0 {
		return nil
	}

	return n
}
