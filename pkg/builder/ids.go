package builder

import (
	"crypto/sha1" //nolint:gosec // UUIDv5 is defined over SHA-1
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// Namespaces for UUIDv5 generation. The product namespace is the one the
// specification names for a reproducible product code (section 11.1).
const (
	namespaceProduct   = "65752A03-A1B0-4D1D-A49C-CD1AEEB2D2F8"
	namespaceComponent = "9B4E1E7C-3F6A-4C2B-8D1E-5A7F0B9C2D31"
	namespacePackage   = "D1F3A6B2-7C4E-4F8A-9B0D-2E5C8A1F3B47"
)

func guidBytes(guid string) [16]byte {
	h := strings.NewReplacer("{", "", "}", "", "-", "").Replace(guid)

	b, err := hex.DecodeString(h)
	if err != nil || len(b) != 16 {
		panic("bad GUID " + guid)
	}

	return [16]byte(b)
}

// uuid5 returns UUIDv5(namespace, name) as a braced upper-case GUID.
func uuid5(namespace, name string) string {
	ns := guidBytes(namespace)
	h := sha1.New() //nolint:gosec // UUIDv5 is defined over SHA-1
	h.Write(ns[:])
	h.Write([]byte(name))
	sum := h.Sum(nil)
	sum[6] = sum[6]&0x0F | 0x50
	sum[8] = sum[8]&0x3F | 0x80

	return strings.ToUpper(fmt.Sprintf("{%x-%x-%x-%x-%x}", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16]))
}

// generatedID builds a deterministic identifier from an identity, as
// section 11.2 describes: prefix + 32 upper-case hex digits of SHA-256.
func generatedID(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))

	return prefix + strings.ToUpper(hex.EncodeToString(sum[:16]))
}

const shortAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789_-"

// shortName derives an 8.3 name for longName in the scope of parentKey,
// unique among taken (lower-case names), following section 11.3 steps 1–4.
// Directories get no extension.
func shortName(parentKey, longName string, directory bool, taken map[string]bool) string { //nolint:cyclop // one pass over the format
	ext := ""

	if !directory { //nolint:nestif // reads best as one block
		if i := strings.LastIndexByte(longName, '.'); i >= 0 && i < len(longName)-1 {
			var extBuilder strings.Builder

			for _, r := range strings.ToLower(longName[i+1:]) {
				if r > 0x20 && r < 0x7F && !strings.ContainsRune(`\/:*?"<>|+,;=[]. `, r) {
					extBuilder.WriteRune(r)
				}

				if extBuilder.Len() == 3 {
					break
				}
			}

			ext = extBuilder.String()
		}
	}

	for n := 0; ; n++ {
		h := sha256.Sum256([]byte(strings.ToLower(parentKey) + "\x1f" + strings.ToLower(longName) + "\x1f" + strconv.Itoa(n)))

		var nameBuilder strings.Builder

		for i := range 8 {
			nameBuilder.WriteByte(shortAlphabet[int(h[i])%len(shortAlphabet)])
		}

		candidate := nameBuilder.String()
		if ext != "" {
			candidate += "." + ext
		}

		if !taken[strings.ToLower(candidate)] {
			taken[strings.ToLower(candidate)] = true

			return candidate
		}
	}
}

// filename returns the DefaultDir/FileName value for a long name: the long
// name alone when it is a valid short name, else short|long.
func filename(parentKey, long string, directory bool, taken map[string]bool, validShort func(string) bool) string {
	if short, l, ok := strings.Cut(long, "|"); ok {
		taken[strings.ToLower(short)] = true

		return short + "|" + l
	}

	if validShort(long) {
		taken[strings.ToLower(long)] = true

		return long
	}

	return shortName(parentKey, long, directory, taken) + "|" + long
}

// taken returns the short names taken in the directory dirKey. The
// directories and files in one directory share it: section 11.3 makes a
// short name unique among the names in its directory.
func (b *build) taken(dirKey string) map[string]bool {
	if b.shortNames[dirKey] == nil {
		b.shortNames[dirKey] = make(map[string]bool)
	}

	return b.shortNames[dirKey]
}

// configuredFileName returns the configured name of a file, or its long name.
func configuredFileName(file *fileInfo) string {
	if file.cfg.Name != "" {
		return file.cfg.Name
	}

	return file.LongName
}

// reserveShortNames takes the literal short names first, the short part of
// a short|long name and a long name that is itself a valid short name, so
// that no generated short name can take one of them, whichever row comes
// first. Two different names with the same literal short name in one
// directory are an error: with SHORTFILENAMES=1 both resolve to it, so one
// file overwrites the other, two directories merge, and a directory beside
// such a file fails (FILE-07).
func (b *build) reserveShortNames() error {
	owners := make(map[string]map[string]string)

	for _, dir := range b.dirOrder {
		if !dir.standard {
			b.reserveShortName(owners, dir.parent, dir.long, dir.namePath)
		}
	}

	for _, comp := range b.cfg.Components {
		info := b.components[comp.ID]
		path := info.cfg.Path(info.index)

		for fileIndex, cfgFile := range comp.Files {
			field := "name"
			if cfgFile.Name == "" {
				field = "source"
			}

			file := b.fileByKey[config.FileKey(cfgFile)]
			b.reserveShortName(owners, info.dir.id, configuredFileName(file), fmt.Sprintf("%s.files[%d].%s", path, fileIndex, field))
		}
	}

	return nil
}

// reserveShortName takes the literal short name of name, if it has one, in
// dirKey. owners maps each directory's literal short names, in lower case,
// to the long names that took them.
func (b *build) reserveShortName(owners map[string]map[string]string, dirKey, name, path string) {
	short, long, ok := strings.Cut(name, "|")
	if !ok {
		if !config.ValidShortName(name) {
			return
		}

		short, long = name, name
	}

	key := strings.ToLower(short)

	if owners[dirKey] == nil {
		owners[dirKey] = make(map[string]string)
	}

	if owner, dup := owners[dirKey][key]; dup && !strings.EqualFold(owner, long) {
		b.errorf(path, "short name %s is also the short name of %q in directory %s; short names must be unique in a directory (FILE-07)",
			short, owner, dirKey)

		return
	}

	owners[dirKey][key] = long
	b.taken(dirKey)[key] = true
}
