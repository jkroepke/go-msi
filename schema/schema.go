// Package schema defines the standard Windows Installer tables that go-msi
// writes: their columns in .idt notation and the _Validation metadata of
// every column (docs/spec/MSISPEC.md section 6). The definitions follow the
// published table reference and the toolchain conventions the specification
// records (File.Sequence and Media.LastSequence are i4, the 26-name
// Category set).
package schema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jkroepke/go-msi/msidb"
)

// ColumnDef is a column with its _Validation row.
type ColumnDef struct {
	Name        string
	Type        msidb.Type
	Category    string
	KeyTable    string
	KeyColumn   int
	MinValue    *int
	MaxValue    *int
	Set         string
	Description string
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
const Categories = "Text;Formatted;Template;Condition;Guid;Path;Version;Language;Identifier;Binary;UpperCase;LowerCase;Filename;Paths;AnyPath;WildCardFilename;RegPath;CustomSource;Property;Cabinet;Shortcut;DefaultDir;FormattedSDDLText;Integer;DoubleInteger;TimeDate"

// Each line: Name|Type|K (key) or empty|Category|KeyTable|KeyColumn|Min|Max|Set|Description.
var defs = map[string][]string{
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
		"Target|S255||Formatted||||||Excecution parameter, depends on the type of custom action",
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
		"Component_|s72||Identifier|Component|1||||Foreign key into the Component table referencing component that controls the installing of the environmental value.",
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
	"Media": {
		"DiskId|i2|K||||1|32767||Primary key, integer to determine sort order for table.",
		"LastSequence|i4|||||0|2147483647||File sequence number for the last file for this media.",
		"DiskPrompt|L64||Text||||||Disk name: the visible text actually printed on the disk.",
		"Cabinet|S255||Cabinet||||||If some or all of the files stored on the media are compressed in a cabinet, the name of that cabinet.",
		"VolumeLabel|S32||Text||||||The label attributed to the volume.",
		"Source|S72||Property||||||The property defining the location of the cabinet file.",
	},
	"MsiFileHash": {
		"File_|s72|K|Identifier|File|1||||Primary key, foreign key into File table referencing file with this hash",
		"Options|i2|||||0|32767||Various options and attributes for this hash.",
		"HashPart1|i4||||||||Size of file in bytes (long integer).",
		"HashPart2|i4||||||||Size of file in bytes (long integer).",
		"HashPart3|i4||||||||Size of file in bytes (long integer).",
		"HashPart4|i4||||||||Size of file in bytes (long integer).",
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
	"Registry": {
		"Registry|s72|K|Identifier||||||Primary key, non-localized token.",
		"Root|i2|||||-1|3||The predefined root key for the registry value, one of rrkEnum.",
		"Key|l255||RegPath||||||The key for the registry value.",
		"Name|L255||Formatted||||||The registry value name.",
		"Value|L0||Formatted||||||The registry value.",
		"Component_|s72||Identifier|Component|1||||Foreign key into the Component table referencing component that controls the installing of the registry value.",
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

var sequenceColumns = []string{
	"Action|s72|K|Identifier||||||Name of action to invoke, either in the engine or the handler DLL.",
	"Condition|S255||Condition||||||Optional expression which skips the action if evaluates to expFalse.If the expression syntax is invalid, the engine will terminate, returning iesBadActionData.",
	"Sequence|I2|||||-4|32767||Number that determines the sort order in which the actions are to be executed.  Leave blank to suppress action.",
}

var tables = map[string]*TableDef{}

func init() {
	for name, lines := range defs {
		def := &TableDef{Name: name}

		for _, line := range lines {
			f := strings.Split(line, "|")
			if len(f) != 10 {
				panic(fmt.Sprintf("schema: %s: malformed column %q", name, line))
			}

			c := ColumnDef{Name: f[0], Type: msidb.MustType(f[1]), Category: f[3], KeyTable: f[4], Set: f[8], Description: f[9]}
			c.Type.Key = f[2] == "K"

			if f[5] != "" {
				c.KeyColumn, _ = strconv.Atoi(f[5])
			}

			c.MinValue = optInt(f[6])
			c.MaxValue = optInt(f[7])
			def.Columns = append(def.Columns, c)
		}

		tables[name] = def
	}
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

	sort.Strings(out)

	return out
}

// Validation builds the _Validation table for the given tables: one row per
// column of every table, _Validation itself included, plus the two
// _SummaryInformation rows the toolchains write. Custom tables (not in this
// package) get rows derived from their column types.
func Validation(present []*msidb.Table) *msidb.Table {
	v := New("_Validation")

	names := map[string]*msidb.Table{}
	for _, t := range present {
		names[t.Name] = t
	}

	names["_Validation"] = v

	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}

	sort.Strings(sorted)

	for _, name := range sorted {
		t := names[name]

		if def, ok := tables[name]; ok {
			for _, c := range def.Columns {
				v.Add(name, c.Name, nullable(c.Type), optCell(c.MinValue), optCell(c.MaxValue), orNil(c.KeyTable), keyCol(c.KeyColumn), orNil(c.Category), orNil(c.Set), orNil(c.Description))
			}

			continue
		}

		for _, c := range t.Columns {
			var category any

			var minV, maxV any

			switch c.Type.Kind {
			case msidb.KindString:
				category = "Text"
			case msidb.KindBinary:
				category = "Binary"
			case msidb.KindInt16:
				minV, maxV = msidb.MinInt16, msidb.MaxInt16
			case msidb.KindInt32:
				minV, maxV = msidb.MinInt32, msidb.MaxInt32
			}

			v.Add(name, c.Name, nullable(c.Type), minV, maxV, nil, nil, category, nil, nil)
		}
	}

	v.Add("_SummaryInformation", "PropertyId", "N", nil, nil, nil, nil, nil, nil, nil)
	v.Add("_SummaryInformation", "Value", "N", nil, nil, nil, nil, "Text", nil, nil)

	return v
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
