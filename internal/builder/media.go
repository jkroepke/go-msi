package builder

import (
	"context"
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
	"github.com/jkroepke/msi-builder/pkg/sign"
)

// maxEmbedded is the most the embedded cabinets may add up to: a package
// above 2 GB is not installable (section 5), and the compound file writer
// stops below it.
const maxEmbedded = 0x7FFFFE00

// maxDiskID is the largest Media.DiskId, an i2 column.
const maxDiskID = 32767

// cabinetSigner is the MsiDigitalCertificate row of the certificate that
// signs the external cabinets.
const cabinetSigner = "CabinetSigner"

// cabinetInfo is one cabinet: a run of payload files in sequence order with
// a Media row of its own (section 5).
type cabinetInfo struct {
	// target holds the written cabinet until the package is committed.
	target spool.Target
	name   string
	files  []*fileInfo
	// digest is what the cabinet's signature covers, once it is signed.
	digest []byte
	size   int64
}

// cabOptions are the cabinet writer options of package.compression.
func (b *build) cabOptions() cab.Options {
	opts := cab.Options{Compression: cab.MSZIP, SignatureReserve: b.signsCabinets()}

	if window, ok := config.LZXWindow(b.cfg.Package.Compression); ok {
		opts.Compression, opts.Window = cab.LZX, window
	} else if b.cfg.Package.Compression == "none" {
		opts.Compression = cab.None
	}

	return opts
}

// signsCabinets reports whether the cabinets get signatures of their own:
// only external ones do, since Windows Installer checks no other
// (section 12).
func (b *build) signsCabinets() bool {
	return b.opts.SignCabinets != nil && !b.cfg.Package.CabinetsEmbedded()
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
// of data fit one cabinet of at most limit bytes, counting the signature
// reserve whether or not the cabinet is signed. A signature follows
// cbCabinet and is not counted.
func fits(files int, nameBytes, folderBytes int64, comp cab.Compression, limit int64) bool {
	return files <= cab.MaxFiles && folderBytes <= cab.MaxFolder &&
		cab.Bound(files, nameBytes, folderBytes, comp)+cab.SignatureReserve <= limit
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

// writeSignatures adds the MsiDigitalCertificate row of the signing
// certificate and an MsiDigitalSignature row per external cabinet when the
// cabinets are signed. Each Hash is filled in once its cabinet is signed.
func (b *build) writeSignatures() {
	if !b.signsCabinets() || len(b.cabinets) == 0 {
		return
	}

	if len(b.opts.SignCabinets.Certificates) == 0 {
		b.errorf("package.embedCabinets", "signing the cabinets needs a certificate")

		return
	}

	b.table("MsiDigitalCertificate").Add(cabinetSigner, b.opts.SignCabinets.Certificates[0].Raw)

	signatures := b.table("MsiDigitalSignature")

	for i := range b.cabinets {
		signatures.Add("Media", strconv.Itoa(i+1), cabinetSigner, nil)
	}
}

// signCabinet signs the external cabinet in its spool and records its digest
// as the Hash of its MsiDigitalSignature row, the index-th.
func (b *build) signCabinet(cabinet *cabinetInfo, index int) error {
	// The build has no context; a timestamp request is bounded by the
	// HTTP client's timeout.
	digest, err := sign.SignCabinet(context.Background(), cabinet.target, *b.opts.SignCabinets)
	if err != nil {
		return fmt.Errorf("cabinet %s: %w", cabinet.name, err)
	}

	size, err := spool.Size(cabinet.target)
	if err != nil {
		return fmt.Errorf("cabinet %s: %w", cabinet.name, err)
	}

	cabinet.digest, cabinet.size = digest, size
	b.tables["MsiDigitalSignature"].Rows[index][3] = digest

	return nil
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
