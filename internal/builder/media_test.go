package builder_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/cab"
	"github.com/jkroepke/msi-builder/internal/cfb"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
	"github.com/jkroepke/msi-builder/pkg/sign"
)

// splitConfig is the minimal example with three more payload files of
// 40000 bytes each.
func splitConfig(t *testing.T) *config.Config {
	t.Helper()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	inputs := map[string][]byte{"hello.exe": exe}

	for i := range 3 {
		inputs[fmt.Sprintf("data%d.bin", i)] = bytes.Repeat([]byte{byte('a' + i)}, 40000)
	}

	cfg := example(t, "minimal", nil, inputs)
	cfg.Package.Compression = "mszip"

	for i := range 3 {
		cfg.Components[0].Files = append(cfg.Components[0].Files, &config.File{Source: fmt.Sprintf("data%d.bin", i)})
	}

	return cfg
}

// mediaRows returns the Media rows as (DiskId, LastSequence, Cabinet).
func mediaRows(t *testing.T, database *msidb.Database) [][3]any {
	t.Helper()

	media := rows(t, database, "Media")
	out := make([][3]any, 0, len(media))

	for _, row := range media {
		out = append(out, [3]any{row[0], row[1], row[3]})
	}

	return out
}

// TestSplitCabinets checks that a payload above Options.CabinetSize is split
// at file boundaries into cabinets with a Media row each, numbered after
// package.cabinet, and that a file larger than the cap gets a cabinet of
// its own.
func TestSplitCabinets(t *testing.T) {
	t.Parallel()

	cfg := splitConfig(t)

	// The files are packed in sequence order, data0.bin to data2.bin and
	// then hello.exe. MSZIP counts each 32 KiB block of the folder as
	// 32788 bytes: two data files take three blocks and fit into 100000
	// bytes, the third would make four.
	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, CabinetSize: 100000})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	database, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	want := [][3]any{{1, 2, "#Hello.cab"}, {2, 4, "#Hello2.cab"}}
	if got := mediaRows(t, database); !slices.Equal(got, want) {
		t.Errorf("Media rows %v, want %v", got, want)
	}

	var names []string

	for _, stream := range database.Streams {
		if strings.HasSuffix(stream.Name, ".cab") {
			names = append(names, stream.Name)
		}
	}

	slices.Sort(names)

	if !slices.Equal(names, []string{"Hello.cab", "Hello2.cab"}) {
		t.Errorf("streams %v", names)
	}

	if res.Cabinets != nil {
		t.Errorf("embedded cabinets listed as external: %+v", res.Cabinets)
	}

	// A cap below one file puts every file into a cabinet of its own.
	res, err = builder.Build(cfg, builder.Options{BuildTime: buildTime, CabinetSize: 1})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	database, err = msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	want = [][3]any{{1, 1, "#Hello.cab"}, {2, 2, "#Hello2.cab"}, {3, 3, "#Hello3.cab"}, {4, 4, "#Hello4.cab"}}
	if got := mediaRows(t, database); !slices.Equal(got, want) {
		t.Errorf("Media rows %v, want %v", got, want)
	}
}

// TestExternalCabinets checks package.embedCabinets false: the Media rows
// name the cabinets without #, the package has no cabinet stream, and the
// cabinets are returned in memory or written beside the package.
func TestExternalCabinets(t *testing.T) {
	t.Parallel()

	cfg := splitConfig(t)
	cfg.Package.EmbedCabinets = new(false)

	opts := builder.Options{BuildTime: buildTime, CabinetSize: 100000}

	res, err := builder.Build(cfg, opts)
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	database, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	want := [][3]any{{1, 2, "Hello.cab"}, {2, 4, "Hello2.cab"}}
	if got := mediaRows(t, database); !slices.Equal(got, want) {
		t.Errorf("Media rows %v, want %v", got, want)
	}

	for _, stream := range database.Streams {
		if strings.HasSuffix(stream.Name, ".cab") {
			t.Errorf("the package holds the cabinet stream %s", stream.Name)
		}
	}

	if len(res.Cabinets) != 2 {
		t.Fatalf("cabinets %+v", res.Cabinets)
	}

	var keys []string

	for _, cabinet := range res.Cabinets {
		if int64(len(cabinet.Data)) != cabinet.Size {
			t.Errorf("cabinet %s: %d bytes, size %d", cabinet.Name, len(cabinet.Data), cabinet.Size)
		}

		files, err := cab.Read(cabinet.Data)
		if err != nil {
			t.Fatalf("cabinet %s: %v", cabinet.Name, err)
		}

		for _, file := range files {
			keys = append(keys, file.Name)
		}
	}

	wantKeys := make([]string, 0, len(res.Files))
	for _, file := range res.Files {
		wantKeys = append(wantKeys, file.Key)
	}

	if !slices.Equal(keys, wantKeys) {
		t.Errorf("cabinet entries %v, want %v", keys, wantKeys)
	}

	// A build to a file writes the same cabinets beside it and leaves no
	// spool behind.
	out := filepath.Join(t.TempDir(), "hello.msi")
	opts.Output = out

	onDisk, err := builder.Build(cfg, opts)
	if err != nil {
		t.Fatalf("build: %v\n%s", err, onDisk.Diagnostics)
	}

	for i, cabinet := range onDisk.Cabinets {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(out), cabinet.Name))
		if err != nil {
			t.Fatal(err)
		}

		if cabinet.Data != nil || !bytes.Equal(data, res.Cabinets[i].Data) {
			t.Errorf("cabinet %s on disk differs from the in-memory build", cabinet.Name)
		}
	}

	left, _ := os.ReadDir(filepath.Dir(out))
	if len(left) != 3 {
		t.Errorf("the output directory holds %d entries, want the package and two cabinets", len(left))
	}

	for _, ent := range left {
		if strings.HasPrefix(ent.Name(), ".msi-builder-") {
			t.Errorf("the build left the spool %s behind", ent.Name())
		}
	}
}

// TestCabinetNameTooLong checks that a second cabinet whose numbered name
// is no stream name is reported at package.cabinet.
func TestCabinetNameTooLong(t *testing.T) {
	t.Parallel()

	cfg := splitConfig(t)
	cfg.Package.Cabinet = strings.Repeat("c", 58) + ".cab"

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, CabinetSize: 1})
	if err == nil {
		t.Fatal("the build passed")
	}

	if !slices.ContainsFunc(res.Diagnostics, func(d config.Diagnostic) bool {
		return d.Path == "package.cabinet" && strings.Contains(d.Message, "cabinet 2")
	}) {
		t.Errorf("diagnostics %s", res.Diagnostics)
	}
}

// TestExternalCabinetNames checks the names an external cabinet cannot
// take: the package's own, in any case, and names Windows cannot create.
func TestExternalCabinetNames(t *testing.T) {
	t.Parallel()

	cfg := splitConfig(t)
	cfg.Package.EmbedCabinets = new(false)
	cfg.Package.Cabinet = "Hello.MSI"

	out := filepath.Join(t.TempDir(), "hello.msi")

	if _, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, Output: out}); err == nil || !strings.Contains(err.Error(), "would replace the package") {
		t.Errorf("a cabinet named like the package gave %v", err)
	}

	if left, _ := os.ReadDir(filepath.Dir(out)); len(left) != 0 {
		t.Errorf("the failed build left %d entries", len(left))
	}

	for _, name := range []string{"NUL.cab", "a.cab.", "..", "a.cab ", "COM.cab", "lpt.cab"} {
		cfg := splitConfig(t)
		cfg.Package.EmbedCabinets = new(false)
		cfg.Package.Cabinet = name

		if !slices.ContainsFunc(config.Validate(cfg), func(d config.Diagnostic) bool { return d.Path == "package.cabinet" }) {
			t.Errorf("cabinet %q passed validation", name)
		}
	}
}

// testSigner is a throwaway code-signing key and certificate.
func testSigner(t *testing.T) *sign.Options {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "msi-builder cabinet test"},
		NotBefore:    buildTime.Add(-time.Hour),
		NotAfter:     buildTime.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return &sign.Options{Signer: key, Certificates: []*x509.Certificate{cert}}
}

// TestSignedExternalCabinets signs each external cabinet and lists the
// certificate and each cabinet's signature value in the signature tables.
func TestSignedExternalCabinets(t *testing.T) {
	t.Parallel()

	signer := testSigner(t)
	cfg := splitConfig(t)
	cfg.Package.EmbedCabinets = new(false)

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, CabinetSize: 100000, SignCabinets: signer})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	database, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	certData := cell(t, database, "MsiDigitalCertificate", "CertData", func([]any) bool { return true })
	if !bytes.Equal(certData.([]byte), signer.Certificates[0].Raw) { //nolint:forcetypeassert // a v0 column
		t.Error("MsiDigitalCertificate.CertData is not the signing certificate")
	}

	signatures := rows(t, database, "MsiDigitalSignature")
	if len(res.Cabinets) != 2 || len(signatures) != len(res.Cabinets) {
		t.Fatalf("%d cabinets, %d MsiDigitalSignature rows", len(res.Cabinets), len(signatures))
	}

	for i, cabinet := range res.Cabinets {
		sig, err := sign.VerifyCabinet(bytes.NewReader(cabinet.Data))
		if err != nil {
			t.Fatalf("cabinet %s: %v", cabinet.Name, err)
		}

		row := signatures[i]
		if row[0] != "Media" || row[1] != strconv.Itoa(i+1) || row[2] != "CabinetSigner" || !bytes.Equal(row[3].([]byte), sig.Value) { //nolint:forcetypeassert // a V0 column
			t.Errorf("MsiDigitalSignature row %v, want the signature value of %s", row[:3], cabinet.Name)
		}

		if int64(len(cabinet.Data)) != cabinet.Size {
			t.Errorf("cabinet %s: %d bytes, size %d", cabinet.Name, len(cabinet.Data), cabinet.Size)
		}

		if _, err := cab.Read(cabinet.Data); err != nil {
			t.Errorf("cabinet %s: %v", cabinet.Name, err)
		}
	}

	// A build to a file signs the spools that move beside the package.
	out := filepath.Join(t.TempDir(), "hello.msi")

	onDisk, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, CabinetSize: 100000, SignCabinets: signer, Output: out})
	if err != nil {
		t.Fatal(err)
	}

	for _, cabinet := range onDisk.Cabinets {
		file, err := os.Open(cabinet.Path)
		if err != nil {
			t.Fatal(err)
		}

		_, err = sign.VerifyCabinet(file)
		_ = file.Close()

		if err != nil {
			t.Errorf("cabinet %s: %v", cabinet.Path, err)
		}
	}

	// Embedded cabinets are covered by the package signature only.
	embedded, err := builder.Build(splitConfig(t), builder.Options{BuildTime: buildTime, SignCabinets: signer})
	if err != nil {
		t.Fatal(err)
	}

	database, err = msidb.Unmarshal(embedded.Package)
	if err != nil {
		t.Fatal(err)
	}

	if database.Table("MsiDigitalSignature") != nil || database.Table("MsiDigitalCertificate") != nil {
		t.Error("an embedded build has signature tables")
	}
}

// TestSignCabinetsChecksOptions refuses signing options without a
// certificate, or with a key that is not the certificate's, before the
// first cabinet is written.
func TestSignCabinetsChecksOptions(t *testing.T) {
	t.Parallel()

	signer := testSigner(t)
	other := testSigner(t)

	for name, opts := range map[string]*sign.Options{
		"no certificate": {Signer: signer.Signer},
		"another key":    {Signer: other.Signer, Certificates: signer.Certificates},
	} {
		cfg := splitConfig(t)
		cfg.Package.EmbedCabinets = new(false)

		res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, CabinetSize: 100000, SignCabinets: opts})
		if err == nil || res.Package != nil || len(res.Cabinets) != 0 {
			t.Errorf("%s: Build = %v with %d cabinets", name, err, len(res.Cabinets))
		}
	}
}

// TestSignedCabinetsWithLanguages keeps MsiDigitalSignature out of the
// language transforms: they are built before the cabinets are signed, so a
// transform that changed the table would null each language's Hash.
func TestSignedCabinetsWithLanguages(t *testing.T) {
	t.Parallel()

	cfg := desktop(t)
	cfg.Package.EmbedCabinets = new(false)
	cfg.UI.Languages = map[string]*config.Language{"de": nil}

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, CabinetSize: 1, SignCabinets: testSigner(t)})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	if len(res.Database.Storages) != 1 {
		t.Fatalf("%d transform storages, want 1", len(res.Database.Storages))
	}

	for _, name := range []string{"MsiDigitalSignature", "MsiDigitalCertificate"} {
		encoded, err := msidb.EncodeStreamName(name, true)
		if err != nil {
			t.Fatal(err)
		}

		if slices.ContainsFunc(res.Database.Storages[0].Streams, func(s cfb.Source) bool { return s.Name == encoded }) {
			t.Errorf("the language transform changes %s", name)
		}
	}

	signatures := rows(t, res.Database, "MsiDigitalSignature")
	if len(signatures) == 0 {
		t.Fatal("no MsiDigitalSignature rows")
	}

	for _, row := range signatures {
		if hash, ok := row[3].([]byte); !ok || len(hash) == 0 {
			t.Errorf("MsiDigitalSignature row %v has no Hash", row[:3])
		}
	}
}
