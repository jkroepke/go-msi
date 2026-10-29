package builder

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/internal/cab"
	"github.com/jkroepke/msi-builder/internal/cfb"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/spool"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// maxEmbedded is the most the embedded cabinets may add up to: a package
// above 2 GB is not installable (section 5), and the compound file writer
// stops below it.
const maxEmbedded = 0x7FFFFE00

// maxDiskID is the largest Media.DiskId, an i2 column.
const maxDiskID = 32767

// cabinetInfo is one cabinet: a run of payload files in sequence order with
// a Media row of its own (section 5).
type cabinetInfo struct {
	// target holds the written cabinet until the package is committed.
	target spool.Target
	name   string
	files  []*fileInfo
	size   int64
}

// cabOptions are the cabinet writer options of package.compression.
func (b *build) cabOptions() cab.Options {
	if window, ok := config.LZXWindow(b.cfg.Package.Compression); ok {
		return cab.Options{Compression: cab.LZX, Window: window}
	}

	if b.cfg.Package.Compression == "none" {
		return cab.Options{Compression: cab.None}
	}

	return cab.Options{Compression: cab.MSZIP}
}

// splitCabinets puts the payload files, in sequence order, into as few
// cabinets as hold them: a cabinet ends before the file that would take it
// over 65535 files, over 65535 blocks or, with the data compressing as
// badly as it can, over Options.CabinetSize or 2 GiB. Files are never split
// across cabinets (section 16, "Decisions not adopted"). A file that alone
// is above the size limit gets a cabinet of its own and fails to write only
// when its data really does not fit.
func (b *build) splitCabinets() {
	limit := int64(cab.MaxSize)
	if b.opts.CabinetSize > 0 {
		limit = min(limit, b.opts.CabinetSize)
	}

	comp := b.cabOptions().Compression

	var (
		current                *cabinetInfo
		nameBytes, folderBytes int64
	)

	for _, file := range b.files {
		if current != nil && !fits(len(current.files)+1, nameBytes+int64(len(file.Key)), folderBytes+file.Size, comp, limit) {
			current = nil
		}

		if current == nil {
			current = &cabinetInfo{name: cabinetName(b.cfg.Package.Cabinet, len(b.cabinets)+1)}
			b.cabinets = append(b.cabinets, current)
			nameBytes, folderBytes = 0, 0
		}

		current.files = append(current.files, file)
		nameBytes += int64(len(file.Key))
		folderBytes += file.Size
	}

	if len(b.cabinets) > maxDiskID {
		b.errorf("package.cabinet", "the payload needs %d cabinets, but Media.DiskId holds at most %d", len(b.cabinets), maxDiskID)
	}

	if !b.cfg.Package.CabinetsEmbedded() {
		return // an external cabinet is a file, not a stream
	}

	// package.cabinet itself is validated with the config.
	for i, cabinet := range b.cabinets[min(1, len(b.cabinets)):] {
		if err := msidb.CheckStreamName(cabinet.name); err != nil {
			b.errorf("package.cabinet", "the name of cabinet %d, %q, is not a valid stream name (MSIDB-11): %v; use a shorter name", i+2, cabinet.name, err)
		}
	}
}

// fits reports whether files entries with nameBytes of names and folderBytes
// of data fit one cabinet of at most limit bytes.
func fits(files int, nameBytes, folderBytes int64, comp cab.Compression, limit int64) bool {
	return files <= cab.MaxFiles && folderBytes <= cab.MaxFolder && cab.Bound(files, nameBytes, folderBytes, comp) <= limit
}

// cabinetName is the name of the number-th cabinet: package.cabinet for the
// first, then the number before its extension, product2.cab for the second.
func cabinetName(first string, number int) string {
	if number == 1 {
		return first
	}

	ext := filepath.Ext(first)

	return strings.TrimSuffix(first, ext) + strconv.Itoa(number) + ext
}

// writeMedia adds a Media row per cabinet: DiskId from 1 and the last
// sequence of its files (section 11.6). A package without files has one row
// without a cabinet (ICE71).
func (b *build) writeMedia() {
	media := b.table("Media")

	if len(b.cabinets) == 0 {
		media.Add(1, 0, nil, nil, nil, nil)

		return
	}

	prefix := ""
	if b.cfg.Package.CabinetsEmbedded() {
		prefix = "#"
	}

	for i, cabinet := range b.cabinets {
		media.Add(i+1, cabinet.files[len(cabinet.files)-1].Sequence, nil, prefix+cabinet.name, nil, nil)
	}
}

// writeCabinet packs the files of cabinet into target, streaming each file
// from disk.
func (b *build) writeCabinet(target spool.Target, cabinet *cabinetInfo) error {
	entries := make([]cab.Entry, 0, len(cabinet.files))

	for _, entry := range cabinet.files {
		entries = append(entries, cab.Entry{
			Name:       entry.Key,
			Size:       entry.Size,
			Time:       b.opts.BuildTime,
			Attributes: cab.AttrArchive,
			Open:       entry.open,
		})
	}

	size, err := cab.WriteTo(target, entries, b.cabOptions())
	if err != nil {
		return fmt.Errorf("cabinet %s: %w", cabinet.name, err)
	}

	cabinet.target = target
	cabinet.size = size

	return nil
}

// source is the cabinet as a stream of the package.
func (c *cabinetInfo) source() cfb.Source {
	return cfb.Source{
		Name: c.name,
		Size: c.size,
		Open: func() (io.ReadCloser, error) { return io.NopCloser(io.NewSectionReader(c.target, 0, c.size)), nil },
	}
}
