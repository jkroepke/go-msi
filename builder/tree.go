package builder

import (
	"bytes"
	"crypto/md5" //nolint:gosec // MsiFileHash is defined as MD5
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/pe"
)

// Component attribute bits (section 8.4).
const (
	compSourceOnly        = 1
	compOptional          = 2
	compRegistryKeyPath   = 4
	compSharedDLLRefCount = 8
	compPermanent         = 16
	compTransitive        = 64
	compNeverOverwrite    = 128
	comp64Bit             = 256
)

// File attribute bits (section 11.5).
const (
	fileReadOnly = 0x1
	fileHidden   = 0x2
	fileSystem   = 0x4
	fileVital    = 0x200
	fileChecksum = 0x400
)

// standardFolderNames are the fixed 8.3 names of the standard folders
// (section 11.3, the reference profile).
var standardFolderNames = map[string]string{ //nolint:gochecknoglobals // lookup table
	"AdminToolsFolder": "Admin", "AppDataFolder": "AppData", "CommonAppDataFolder": "CommApp",
	"CommonFilesFolder": "CFiles", "CommonFiles64Folder": "CFiles64", "DesktopFolder": "Desktop",
	"FavoritesFolder": "Favs", "FontsFolder": "Fonts", "LocalAppDataFolder": "LocalApp",
	"MyPicturesFolder": "Pictures", "NetHoodFolder": "NetHood", "PersonalFolder": "Personal",
	"PrintHoodFolder": "Printers", "ProgramFilesFolder": "PFiles", "ProgramFiles64Folder": "PFiles64",
	"ProgramMenuFolder": "PMenu", "RecentFolder": "Recent", "SendToFolder": "SendTo",
	"StartMenuFolder": "StrtMenu", "StartupFolder": "StartUp", "SystemFolder": "System",
	"System16Folder": "System16", "System64Folder": "System64", "TempFolder": "Temp",
	"TemplateFolder": "Template", "WindowsFolder": "Windows", "WindowsVolume": ".",
}

func (b *build) identity() error {
	var err error

	b.upgradeCode, err = config.NormalizeGUID(b.cfg.Product.UpgradeCode)
	if err != nil {
		return fmt.Errorf("product.upgradeCode: %w", err)
	}

	b.version, err = config.ParseVersion(b.cfg.Product.Version)
	if err != nil {
		return fmt.Errorf("product.version: %w", err)
	}

	switch b.cfg.Product.ProductCode {
	case config.Auto:
		// Derived later, once every input is known (inputDigest).
	default:
		b.productCode, err = config.NormalizeGUID(b.cfg.Product.ProductCode)
		if err != nil {
			return fmt.Errorf("product.productCode: %w", err)
		}
	}

	if b.cfg.Product.PackageCode != config.Auto {
		b.packageCode, err = config.NormalizeGUID(b.cfg.Product.PackageCode)
		if err != nil {
			return fmt.Errorf("product.packageCode: %w", err)
		}
	}

	return nil
}

// deriveProductCode computes productCode: auto from the upgrade code, the
// platform, the version and a digest of the configuration and every input
// file, so that changed content gives a new code (RUN-11) and unchanged
// content the same one.
func (b *build) deriveProductCode() error {
	if b.productCode != "" {
		return nil
	}

	cfgCopy := *b.cfg
	cfgCopy.BaseDir = ""

	yamlBytes, err := yaml.Marshal(&cfgCopy)
	if err != nil {
		return fmt.Errorf("digest of the configuration: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(yamlBytes)

	// Disabled has no YAML key, so the marshaled config does not show it.
	if b.cfg.Upgrade != nil && b.cfg.Upgrade.Disabled {
		hasher.Write([]byte("upgrade:disabled"))
	}

	for _, f := range b.files {
		sum := sha256.Sum256(f.Data)
		hasher.Write([]byte(f.Key))
		hasher.Write(sum[:])
	}

	for _, name := range sortedStringKeys(b.cfg.Icons) {
		hasher.Write([]byte("icon:" + name))
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	b.productCode = uuid5(namespaceProduct, "product|"+b.upgradeCode+"|"+b.cfg.Product.Version+"|"+string(b.cfg.Package.Platform)+"|"+digest)

	return nil
}

func sortedStringKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.Sort(out)

	return out
}

// directories builds the Directory table.
func (b *build) directories() error {
	b.addDir(&dirInfo{id: targetDir, canonical: targetDir, standard: true})

	var walk func(ds []*config.Directory, parent *dirInfo)

	walk = func(ds []*config.Directory, parent *dirInfo) {
		for _, dir := range ds {
			if existing, dup := b.dirs[dir.ID]; dup {
				// A top-level TARGETDIR node only carries children for the
				// root the builder already added.
				if parent == nil && dir.ID == targetDir {
					walk(dir.Children, existing)
				}

				continue
			}

			info := &dirInfo{id: dir.ID}

			switch {
			case parent == nil && config.IsStandardFolder(dir.ID):
				info.parent = targetDir
				info.standard = true
				info.canonical = dir.ID
			case parent == nil:
				info.parent = targetDir
				info.long = dir.Name
				info.canonical = "TARGETDIR\\" + strings.ToLower(longPart(dir.Name))
			default:
				info.parent = parent.id
				info.long = dir.Name
				info.canonical = parent.canonical + "\\" + strings.ToLower(longPart(dir.Name))
			}

			if dir.Configurable {
				b.secure[dir.ID] = true
			}

			b.addDir(info)
			walk(dir.Children, info)
		}
	}

	walk(b.cfg.Directories, nil)

	return nil
}

func longPart(name string) string {
	if _, long, ok := strings.Cut(name, "|"); ok {
		return long
	}

	return name
}

func (b *build) addDir(d *dirInfo) {
	b.dirs[d.id] = d
	b.dirOrder = append(b.dirOrder, d)
}

// ensureStandardDir adds a standard folder row when an action needs it as a
// working directory.
func (b *build) ensureStandardDir(id string) {
	if _, ok := b.dirs[id]; ok {
		return
	}

	b.addDir(&dirInfo{id: id, parent: targetDir, standard: true, canonical: id})
}

// writeDirectoryTable emits the rows with generated short names. Every
// directory is written: the engine resolves the whole chain.
func (b *build) writeDirectoryTable() {
	tbl := b.table("Directory")
	taken := make(map[string]map[string]bool)

	for _, dir := range b.dirOrder {
		switch {
		case dir.id == targetDir:
			tbl.Add(targetDir, nil, "SourceDir")
		case dir.standard:
			name, ok := standardFolderNames[dir.id]
			if !ok {
				name = "."
			}

			tbl.Add(dir.id, dir.parent, name)
		default:
			if taken[dir.parent] == nil {
				taken[dir.parent] = make(map[string]bool)
			}

			tbl.Add(dir.id, dir.parent, filename(dir.parent, dir.long, true, taken[dir.parent], config.ValidShortName))
		}
	}
}

// writeComponents prepares the component records; rows are written after the
// files are read, because the key path and GUID may depend on them.
func (b *build) writeComponents() error { //nolint:cyclop // one pass over the format; splitting would scatter it
	for i, comp := range b.cfg.Components {
		info := &componentInfo{cfg: comp, index: i, dir: b.dirs[comp.Directory]}
		if info.dir == nil {
			return fmt.Errorf("%s: directory %s is not defined", comp.Path(i), comp.Directory)
		}

		info.dir.used = true

		switch comp.Bitness {
		case "64":
			info.is64 = true
		case "32":
			info.is64 = false
		default:
			info.is64 = b.is64
		}

		if info.is64 {
			info.attributes |= comp64Bit
		}

		if comp.Permanent {
			info.attributes |= compPermanent
		}

		if comp.Transitive {
			info.attributes |= compTransitive
		}

		if comp.NeverOverwrite {
			info.attributes |= compNeverOverwrite
		}

		if comp.SharedDLLRefCount {
			info.attributes |= compSharedDLLRefCount
		}

		b.components[comp.ID] = info
	}

	return nil
}

// inputs reads every payload file and derives version, language, hash and
// attributes, then writes the Component, File, MsiFileHash, CreateFolder,
// Registry, Environment and service tables.
func (b *build) inputs() error { //nolint:cyclop // one pass over the format; splitting would scatter it
	for _, c := range b.cfg.Components {
		info := b.components[c.ID]
		p := info.cfg.Path(info.index)

		for j, f := range c.Files {
			filePath := fmt.Sprintf("%s.files[%d]", p, j)

			file := b.readFile(filePath, f, info)
			if file == nil {
				continue
			}

			if _, dup := b.fileByKey[file.Key]; dup {
				b.errorf(filePath+".id", "file key %s is used twice", file.Key)

				continue
			}

			b.fileByKey[file.Key] = file
			b.files = append(b.files, file)
		}
	}

	if err := b.diags.Err(); err != nil {
		return err //nolint:wrapcheck // the diagnostics carry their own paths
	}

	// Companion files: Version names another File key.
	for _, fi := range b.files {
		if v, ok := fi.Version.(string); ok && fi.cfg.Version != config.Auto && fi.cfg.Version != config.None {
			if _, err := config.ParseVersion(v); err != nil {
				if _, exists := b.fileByKey[v]; !exists {
					b.errorf(fi.Component.cfg.Path(fi.Component.index)+".files", "companion file %s is not defined", v)
				}
			}
		}
	}

	return b.diags.Err() //nolint:wrapcheck // the diagnostics carry their own paths
}

func (b *build) readFile(diagPath string, cfgFile *config.File, comp *componentInfo) *fileInfo { //nolint:cyclop,gocognit // one pass over the format
	path := cfgFile.Source
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.cfg.BaseDir, filepath.FromSlash(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		b.errorf(diagPath+".source", "cannot read %s: %v", cfgFile.Source, err)

		return nil
	}

	if len(data) > 0x7FFFFFFF {
		b.errorf(diagPath+".source", "%s is larger than 2 GB", cfgFile.Source)

		return nil
	}

	file := &fileInfo{
		Key:       config.FileKey(cfgFile),
		Component: comp,
		cfg:       cfgFile,
		path:      path,
		Data:      data,
		LongName:  config.FileLongName(cfgFile),
	}

	info, perr := pe.Parse(bytes.NewReader(data))
	file.isPE = perr == nil

	switch cfgFile.Version {
	case config.Auto:
		if file.isPE { //nolint:nestif // reads best as one block
			if !info.HasVersion {
				if !cfgFile.AllowUnversionedExecutable {
					b.errorf(diagPath+".version", "%s is a PE file without a version resource; an upgrade can delete it (RUN-02). "+
						"Set allowUnversionedExecutable: true if this is intended", file.LongName)
				}
			} else {
				file.Version = info.FileVersion
			}

			if info.Machine == pe.MachineI386 && comp.is64 {
				b.warnf(diagPath+".source", "%s is a 32-bit executable in a 64-bit component", file.LongName)
			}
		}
	case config.None:
	default:
		file.Version = cfgFile.Version
	}

	switch cfgFile.Language {
	case config.Auto:
		if file.isPE && info.HasVersion && len(info.Languages) > 0 {
			file.Language = info.Language()
		}
	case config.None:
	default:
		file.Language = cfgFile.Language
	}

	if file.Version == nil {
		file.Hash = fileHash(data)
	}

	if *cfgFile.Vital {
		file.Attrs |= fileVital
	}

	if cfgFile.Checksum {
		if file.isPE && info.HasChecksum {
			file.Attrs |= fileChecksum
		} else {
			b.warnf(diagPath+".checksum", "%s has no PE checksum to verify; the checksum attribute is not set", file.LongName)
		}
	}

	if cfgFile.ReadOnly {
		file.Attrs |= fileReadOnly
	}

	if cfgFile.Hidden {
		file.Attrs |= fileHidden
	}

	if cfgFile.System {
		file.Attrs |= fileSystem
	}

	return file
}

// fileHash computes the MsiFileHash words: MD5 as four little-endian int32,
// zeros for an empty file (RUN-12).
func fileHash(data []byte) *[4]int32 {
	var out [4]int32

	if len(data) == 0 {
		return &out
	}

	sum := md5.Sum(data) //nolint:gosec // MsiFileHash is defined as MD5
	for i := range out {
		out[i] = int32(binary.LittleEndian.Uint32(sum[4*i:])) //nolint:gosec // the hash is stored as four signed words
	}

	return &out
}

// sequenceFiles orders the files (canonical directory, name, key), numbers
// them and writes the file-related tables and the Component table.
func (b *build) sequenceFiles() error { //nolint:cyclop // one pass over the format; splitting would scatter it
	slices.SortStableFunc(b.files, func(left, right *fileInfo) int {
		if left.Component.dir.canonical != right.Component.dir.canonical {
			return strings.Compare(left.Component.dir.canonical, right.Component.dir.canonical)
		}

		if left.LongName != right.LongName {
			return strings.Compare(left.LongName, right.LongName)
		}

		return strings.Compare(left.Key, right.Key)
	})

	taken := make(map[string]map[string]bool)

	for i, file := range b.files {
		file.Sequence = i + 1

		dir := file.Component.dir.id
		if taken[dir] == nil {
			taken[dir] = make(map[string]bool)
		}

		name := file.cfg.Name
		if name == "" {
			name = file.LongName
		}

		file.FileName = filename(dir, name, false, taken[dir], config.ValidShortName)
	}

	if err := b.deriveProductCode(); err != nil {
		return err
	}

	b.writeDirectoryTable()

	// Components in config order; rows are sorted by msidb anyway.
	for _, comp := range b.cfg.Components {
		info := b.components[comp.ID]
		path := info.cfg.Path(info.index)

		if err := b.resolveKeyPath(path, info); err != nil {
			return err
		}

		if err := b.componentGUID(path, info); err != nil {
			return err
		}

		b.table("Component").Add(comp.ID, info.guid, comp.Directory, info.attributes, nullable(comp.Condition), info.keyPath)

		if info.keyPath == nil || comp.CreateFolder {
			b.table("CreateFolder").Add(comp.Directory, comp.ID)
		}

		b.writeRegistry(info)
		b.writeEnvironment(info)
		b.writeServices(info)
	}

	for _, fi := range b.files {
		b.table("File").Add(fi.Key, fi.Component.cfg.ID, fi.FileName, len(fi.Data), fi.Version, fi.Language, fi.Attrs, fi.Sequence)

		if fi.Hash != nil {
			b.table("MsiFileHash").Add(fi.Key, 0, int(fi.Hash[0]), int(fi.Hash[1]), int(fi.Hash[2]), int(fi.Hash[3]))
		}
	}

	media := b.table("Media")
	if len(b.files) > 0 {
		media.Add(1, len(b.files), nil, "#"+b.cfg.Package.Cabinet, nil, nil)
	} else {
		media.Add(1, 0, nil, nil, nil, nil)
	}

	return nil
}

// resolveKeyPath picks the component's key path: the explicit one, else the
// first file, else the first registry value, else the directory.
func (b *build) resolveKeyPath(path string, info *componentInfo) error { //nolint:cyclop // one pass over the format; splitting would scatter it
	comp := info.cfg

	var files []*fileInfo

	for _, f := range b.files {
		if f.Component == info {
			files = append(files, f)
		}
	}

	byRef := func(ref string) *fileInfo {
		for _, f := range files {
			if f.Key == ref || f.LongName == ref {
				return f
			}
		}

		return nil
	}

	switch keyPath := comp.KeyPath; {
	case keyPath == "directory":
		info.keyPath = nil
	case keyPath == "":
		if len(files) > 0 {
			info.keyFile = files[0]
		} else if len(comp.Registry) > 0 {
			info.keyPath = b.registryID(info, comp.Registry[0])
			info.attributes |= compRegistryKeyPath
		}
	case byRef(keyPath) != nil:
		info.keyFile = byRef(keyPath)
	default:
		for _, r := range comp.Registry {
			if r.ID == keyPath {
				info.keyPath = keyPath
				info.attributes |= compRegistryKeyPath
			}
		}

		if info.keyPath == nil {
			return fmt.Errorf("%s.keyPath: %q is neither a file nor a registry id", path, keyPath)
		}
	}

	if info.keyFile != nil {
		info.keyPath = info.keyFile.Key

		// A companion file never is a key path (section 8.6).
		if v, ok := info.keyFile.Version.(string); ok {
			if _, err := config.ParseVersion(v); err != nil {
				b.errorf(path+".keyPath", "the key file %s is a companion file of %s", info.keyFile.Key, v)
			}
		}
	}

	if len(comp.Services) > 0 && info.keyFile == nil {
		b.errorf(path+".services", "a component with services needs the service executable as its key file")
	}

	return nil
}

// componentGUID derives the component code: pinned, none, or UUIDv5 over the
// upgrade code, the bitness and the key path's canonical location.
func (b *build) componentGUID(path string, info *componentInfo) error { //nolint:cyclop // one pass over the format; splitting would scatter it
	comp := info.cfg

	switch comp.GUID {
	case config.None:
		info.guid = nil

		return nil
	case config.Auto:
	default:
		g, err := config.NormalizeGUID(comp.GUID)
		if err != nil {
			return fmt.Errorf("%s.guid: %w", path, err)
		}

		info.guid = g

		return nil
	}

	bitness := "32"
	if info.is64 {
		bitness = "64"
	}

	var location string

	switch {
	case info.keyFile != nil:
		location = "file|" + info.dir.canonical + "\\" + strings.ToLower(info.keyFile.LongName)
	case info.keyPath != nil:
		for _, r := range comp.Registry {
			if b.registryID(info, r) == info.keyPath {
				location = "registry|" + strings.ToLower(r.Root+"\\"+r.Key+"\\"+r.Name)
			}
		}
	default:
		location = "directory|" + info.dir.canonical
	}

	info.guid = uuid5(namespaceComponent, b.upgradeCode+"|"+bitness+"|"+location)

	return nil
}

// features writes Feature and FeatureComponents.
func (b *build) features() error {
	tbl := b.table("Feature")
	featureComps := b.table("FeatureComponents")
	display := 0

	config.WalkFeatures(b.cfg.Features, func(feature, parent *config.Feature) {
		var parentID any
		if parent != nil {
			parentID = parent.ID
		}

		display += 2

		var disp any

		switch feature.Display {
		case "hidden":
			disp = 0
		case "expanded":
			disp = display + 1
		default:
			disp = display
		}

		display++

		attrs := 0

		if !*feature.AllowAbsent {
			attrs |= 16 // UIDisallowAbsent
		}

		if !feature.AllowAdvertise {
			attrs |= 8 // DisallowAdvertise
		}

		tbl.Add(feature.ID, parentID, nullable(feature.Title), nullable(feature.Description), disp, *feature.Level, nullable(feature.Directory), attrs)

		for _, c := range feature.Components {
			featureComps.Add(feature.ID, c.ID)
		}
	})

	return nil
}
