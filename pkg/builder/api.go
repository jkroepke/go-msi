// Package builder turns a validated config.Config into an .msi package: it
// derives every table row, numbers the sequences, packs the cabinet, writes
// the database and verifies the result by reading it back.
//
// The rules come from docs/spec/MSISPEC.md and the Windows results in
// docs/spec/MSISPEC-RESULTS.md; docs/design/architecture.md describes the
// pipeline. The implementation lives in internal/builder; this package is
// the stable API in front of it.
package builder

import (
	"time"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
	"github.com/jkroepke/msi-builder/pkg/sign"
)

// Options control a build.
type Options struct {
	BuildTime time.Time
	// SignCabinets signs each cabinet written beside the package
	// (package.embedCabinets false) with Authenticode and lists the
	// signing certificate and each cabinet's signature value in the
	// MsiDigitalCertificate and MsiDigitalSignature tables, so that Windows
	// Installer refuses a cabinet that was changed or swapped. The package
	// itself is not signed: sign it with sign.Sign after the build, since
	// the tables protect the cabinets only as far as the package is
	// protected. Nil, or embedded cabinets, which the package signature
	// covers, leave the cabinets unsigned. A timestamp request is bounded
	// by the HTTP client's timeout; the build cannot cancel it.
	SignCabinets *sign.Options
	ToolName     string
	// Output is the path the package is written to. The cabinet and the
	// package are written to temporary files beside it and the payload
	// files stream through, so the build needs memory for the tables only,
	// not for the files. The package replaces a file at Output only once it
	// is verified; a failed build leaves nothing behind. When Output is
	// empty the package is built in memory and returned in Result.Package.
	Output string
	// CabinetSize caps the size of each cabinet in bytes, counted with
	// every block at the largest size its compression allows (no gain for
	// stored, slightly more than the data for MSZIP, 19 % more for LZX);
	// 0 or less means the 2 GiB a cabinet holds. A file larger than the cap
	// gets a cabinet of its own.
	CabinetSize int64
	CFBVersion  int
	SkipVerify  bool
}

// Result is a finished build.
type Result struct {
	// Package holds the bytes when Options.Output was empty.
	Package []byte
	// Cabinets are the cabinets written beside the package
	// (package.embedCabinets false), in DiskId order. After a build to a
	// file they are in the package's folder (that of a symlink's target
	// when Output is a symlink) and Data is nil.
	Cabinets    []Cabinet
	Diagnostics config.Diagnostics
	ProductCode string
	PackageCode string
	// Files lists every payload file in File.Sequence order.
	Files []File
	// Size is the package length in bytes.
	Size int64
}

// Cabinet is a cabinet written beside the package.
type Cabinet struct {
	// Name is the file name, Media.Cabinet.
	Name string
	// Path is where a build to a file wrote the cabinet.
	Path string
	// Data holds the bytes when Options.Output was empty.
	Data []byte
	// Size is the cabinet length in bytes.
	Size int64
}

// File describes one payload file of a finished build.
type File struct {
	// Key is the File table primary key.
	Key string
	// Name is the long file name as installed.
	Name string
	// Source is the source path as written in the config.
	Source string
	// Component is the ID of the component that owns the file.
	Component string
	// Version is the File.Version value, or "" for an unversioned file.
	Version string
	// Language is the File.Language value, or "" when none is recorded.
	Language string
	// Size is the file size in bytes.
	Size int64
	// Sequence is the File.Sequence value, which orders the files in the
	// cabinet.
	Sequence int
}

// Build builds the package described by cfg. Validation diagnostics of the
// config are included; the returned error lists every error-level finding.
func Build(cfg *config.Config, opts Options) (*Result, error) {
	res, err := builder.Build(cfg, builder.Options(opts))
	if res == nil {
		return nil, err //nolint:wrapcheck // the diagnostics carry their own paths
	}

	files := make([]File, len(res.Files))
	for i, file := range res.Files {
		files[i] = File(file)
	}

	var cabinets []Cabinet
	for _, cabinet := range res.Cabinets {
		cabinets = append(cabinets, Cabinet(cabinet))
	}

	return &Result{
		Package:     res.Package,
		Cabinets:    cabinets,
		Diagnostics: res.Diagnostics,
		ProductCode: res.ProductCode,
		PackageCode: res.PackageCode,
		Files:       files,
		Size:        res.Size,
	}, err //nolint:wrapcheck // the diagnostics carry their own paths
}
