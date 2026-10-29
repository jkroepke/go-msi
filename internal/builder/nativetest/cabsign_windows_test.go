//go:build windows

package nativetest_test

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/cab"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/pkg/sign"
)

var (
	pMsiGetFileSignatureInformationW = msiDLL.NewProc("MsiGetFileSignatureInformationW")
	pCertFreeCertificateContext      = syscall.NewLazyDLL("crypt32.dll").NewProc("CertFreeCertificateContext")
)

// fileSignatureHash returns the hash MsiGetFileSignatureInformation reports
// for the signed file at path, the value MsiDigitalSignature.Hash compares.
// A call without a buffer returns 0 and leaves the size 0 (SIG-08), so it
// tries growing buffers.
func fileSignatureHash(t *testing.T, path string) ([]byte, error) {
	t.Helper()

	var r uintptr

	for _, want := range []uint32{64, 256, 1024, 4096, 65536} {
		hash := make([]byte, want)
		size := want

		if r = signatureInformation(t, path, hash, &size); r == 0 {
			return hash[:size], nil
		}
	}

	return nil, fmt.Errorf("MsiGetFileSignatureInformation: HRESULT %#x up to a 64 KiB buffer", r)
}

// signatureInformation calls MsiGetFileSignatureInformation with hash as the
// buffer and releases the certificate it returns.
func signatureInformation(t *testing.T, path string, hash []byte, size *uint32) uintptr {
	t.Helper()

	var (
		cert uintptr
		buf  uintptr
	)

	if len(hash) > 0 {
		buf = uintptr(unsafe.Pointer(&hash[0]))
	}

	r, _, _ := pMsiGetFileSignatureInformationW.Call(uintptr(unsafe.Pointer(wstr(t, path))), 0,
		uintptr(unsafe.Pointer(&cert)), buf, uintptr(unsafe.Pointer(size)))
	if cert != 0 {
		_, _, _ = pCertFreeCertificateContext.Call(cert)
	}

	return r
}

// logLines returns the lines of the msiexec log at path that name an
// error or the cabinet, for the results.
func logLines(path string) string {
	var out []string

	for line := range strings.SplitSeq(readLog(path), "\n") {
		if strings.Contains(line, "Error 1") || strings.Contains(line, "WinVerifyTrust") || strings.Contains(line, "digital signature") {
			out = append(out, strings.TrimSpace(line))
		}
	}

	return strings.Join(out, "\n")
}

// probeFilesOf are the three small probe files filled with fill.
func probeFilesOf(fill byte) []probeFile {
	files := make([]probeFile, 0, 3)

	for i := range 3 {
		data := bytes.Repeat([]byte{fill + byte(i)}, 40000)
		files = append(files, probeFile{
			name: "data" + string(rune('0'+i)) + ".bin",
			size: int64(len(data)),
			data: func() io.Reader { return bytes.NewReader(data) },
		})
	}

	return files
}

// TestCabinetSignatureFormat checks the cabinet signature against Windows
// (SIG-08): WinVerifyTrust and signtool accept a cabinet msi-builder
// signed, msi-builder accepts one signtool signed, and
// MsiGetFileSignatureInformation reports the digest msi-builder writes to
// MsiDigitalSignature.Hash.
func TestCabinetSignatureFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("trusting the test CA needs elevation")
	}

	pki := newPKI(t, rsaLeafKey(t))
	files := []cab.File{{Name: "a.txt", Data: []byte("signed cabinet")}, {Name: "b.bin", Data: bytes.Repeat([]byte("msi"), 40000)}}

	reserved, err := cab.Write(files, cab.Options{Compression: cab.MSZIP, SignatureReserve: true})
	if err != nil {
		t.Fatal(err)
	}

	var ourSigned []byte

	for _, algo := range []crypto.Hash{crypto.SHA256, crypto.SHA1} {
		path := writeTemp(t, "ours-"+algo.String()+".cab", reserved)

		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}

		value, err := sign.SignCabinet(t.Context(), file, pki.options(algo))
		_ = file.Close()

		if err != nil {
			t.Fatal(err)
		}

		if got := winVerifyTrust(t, path); got != pki.wantValid() {
			t.Errorf("%v: WinVerifyTrust of the msi-builder cabinet signature = %#x, want %#x", algo, got, pki.wantValid())
		}

		if status := authenticodeStatus(t, path); status != "Valid" && pki.trusted {
			t.Errorf("%v: Get-AuthenticodeSignature = %s", algo, status)
		}

		if pki.trusted {
			// The hash Windows reports is the signer's signature value, not
			// the content digest.
			hash, err := fileSignatureHash(t, path)

			switch {
			case err != nil:
				t.Errorf("%v: %v", algo, err)
			case !bytes.Equal(hash, value):
				t.Errorf("%v: MsiGetFileSignatureInformation hash %x, want the signature value %x", algo, hash, value)
			default:
				t.Logf("SIG-08: %v: MsiGetFileSignatureInformation reports the %d-byte signature value", algo, len(hash))
			}
		}

		// A changed data byte breaks the signature for Windows too.
		signed, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		if algo == crypto.SHA256 {
			ourSigned = bytes.Clone(signed)
		}

		signed[len(reserved)-1] ^= 1

		got := winVerifyTrust(t, writeTemp(t, "tampered.cab", signed))
		if got == trustOK {
			t.Errorf("%v: WinVerifyTrust accepted a changed cabinet", algo)
		}

		t.Logf("SIG-08: %v: WinVerifyTrust of a changed data byte = %#x", algo, got)
	}

	signtool := findSigntool()
	if signtool == "" {
		t.Log("SIG-08: signtool.exe not found; skipping the signtool cross-checks")

		return
	}

	if out, err := signtoolVerify(t, signtool, writeTemp(t, "ours.cab", ourSigned)); err != nil {
		t.Errorf("signtool verify of the msi-builder cabinet signature: %v\n%s", err, out)
	}

	// signtool inserts the reserve into a cabinet without one.
	plain, err := cab.Write(files, cab.Options{Compression: cab.MSZIP})
	if err != nil {
		t.Fatal(err)
	}

	viaSigntool := writeTemp(t, "signtool.cab", plain)
	signtoolSign(t, signtool, pki, viaSigntool, "SHA256")

	data, err := os.ReadFile(viaSigntool)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("SIG-08: signtool cabinet: flags % x, reserve fields and signature header % x, %d bytes for a %d-byte cabinet",
		data[0x1E:0x20], data[36:min(len(data), 60)], len(data), len(plain))

	// The indirect data signtool writes for a cabinet.
	for _, oid := range [][]byte{{0x2B, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x02, 0x01, 0x19}, {0x2B, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x02, 0x01, 0x1C}} {
		if at := bytes.Index(data, oid); at >= 0 {
			t.Logf("SIG-08: signtool indirect data from the type OID: % x", data[max(0, at-6):min(len(data), at+90)])
		}
	}

	sig, err := sign.VerifyCabinet(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("sign.VerifyCabinet of the signtool signature: %v", err)
	}

	t.Logf("SIG-08: signtool signed digest %x verified by msi-builder", sig.Digest)
}

// signedProbe builds the probe's three files into three signed external
// cabinets.
func signedProbe(t *testing.T, files []probeFile, opts sign.Options) (string, *builder.Result) {
	t.Helper()

	return buildProbe(t, "  compression: mszip\n  embedCabinets: false", files,
		builder.Options{BuildTime: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC), CabinetSize: 1, SignCabinets: &opts})
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// withHash writes the package at path again beside it as name, with
// MsiDigitalSignature.Hash of the cabinet with DiskId i+1 set to hash(i),
// and returns its path.
func withHash(t *testing.T, path, name string, hash func(i int) []byte) string {
	t.Helper()

	database, err := msidb.Unmarshal(readFile(t, path))
	if err != nil {
		t.Fatal(err)
	}

	tbl := database.Table("MsiDigitalSignature")
	for i, row := range tbl.Rows {
		// A nil []byte in an any is not a null value.
		if value := hash(i); value != nil {
			row[tbl.Column("Hash")] = value
		} else {
			row[tbl.Column("Hash")] = nil
		}
	}

	data, err := database.Marshal(msidb.Options{})
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(filepath.Dir(path), name)
	if err := os.WriteFile(out, data, 0o600); err != nil {
		t.Fatal(err)
	}

	return out
}

// checkInstall installs the package at path, logs the outcome for the
// results (SIG-09) and removes it again. It fails the test unless msiexec
// installed (want true) or refused it without a trace (want false), and
// reports whether the installed files are the package's own.
func checkInstall(t *testing.T, path string, files []probeFile, label string, want bool) bool {
	t.Helper()

	log := filepath.Join(t.TempDir(), "install.log")
	dir := filepath.Join(os.Getenv("ProgramFiles"), "Cabinet Probe")

	code := msiexecCode(t, log, "/i", path)
	if code != 0 {
		t.Logf("SIG-09: %s: refused, msiexec exit %d\n%s", label, code, logLines(log))

		if want {
			t.Errorf("%s: msiexec exit %d, want an installation", label, code)
		}

		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: the failed install left %s: %v", label, dir, err)
		}

		return false
	}

	same := true

	for _, file := range files {
		installed, err := os.ReadFile(filepath.Join(dir, file.name))
		source, _ := io.ReadAll(file.data())

		if err != nil || !bytes.Equal(installed, source) {
			same = false
		}
	}

	t.Logf("SIG-09: %s: installed, files match the package's sources: %v", label, same)
	msiexecQuiet(t, filepath.Join(t.TempDir(), "uninstall.log"), "/x", path)

	if !want {
		t.Errorf("%s: installed, want it refused", label)
	}

	return same
}

// TestMsiexecChecksSignedCabinets installs a package whose three external
// cabinets are signed and listed in MsiDigitalSignature (SIG-09), then
// checks that Windows Installer refuses a changed cabinet, a cabinet signed
// by the same certificate for other content and an unsigned cabinet, and
// that it does not ask for a trusted certificate. Packages with a null Hash and with the
// content digest as Hash show why Hash holds the signature value.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecChecksSignedCabinets(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	pki := newPKI(t, rsaLeafKey(t))
	if !pki.trusted {
		t.Skip("the test CA could not be trusted (needs elevation)")
	}

	files := probeFilesOf('a')
	path, res := signedProbe(t, files, pki.options(crypto.SHA256))
	dir := filepath.Dir(path)

	if len(res.Cabinets) != 3 {
		t.Fatalf("cabinets %+v, want three", res.Cabinets)
	}

	for _, cabinet := range res.Cabinets {
		if got := winVerifyTrust(t, cabinet.Path); got != trustOK {
			t.Errorf("WinVerifyTrust %s = %#x", cabinet.Name, got)
		}
	}

	// The same package with MsiDigitalSignature.Hash null, so only the
	// certificate is compared, and with the content digest as Hash.
	noHash := withHash(t, path, "probe-nohash.msi", func(int) []byte { return nil })
	digestHash := withHash(t, path, "probe-digest.msi", func(i int) []byte {
		digest, err := sign.CabinetDigest(bytes.NewReader(readFile(t, res.Cabinets[i].Path)), crypto.SHA256)
		if err != nil {
			t.Fatal(err)
		}

		return digest
	})

	second := filepath.Join(dir, res.Cabinets[1].Name)

	original, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}

	// The same certificate signs other content in the same layout.
	otherPath, other := signedProbe(t, probeFilesOf('x'), pki.options(crypto.SHA256))
	swapped := filepath.Join(filepath.Dir(otherPath), other.Cabinets[1].Name)

	unsignedPath, unsigned := buildProbe(t, "  compression: mszip\n  embedCabinets: false", files,
		builder.Options{BuildTime: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC), CabinetSize: 1})
	unsignedCab := filepath.Join(filepath.Dir(unsignedPath), unsigned.Cabinets[1].Name)

	// The last byte before cbCabinet is cabinet data, not signature.
	changed := bytes.Clone(original)
	changed[binary.LittleEndian.Uint32(changed[8:])-1] ^= 1

	// want: the signed, changed, swapped and unsigned second cabinet
	// install. A null Hash only checks the certificate, so a cabinet of
	// another build by the same signer installs; the content digest is not
	// what Windows Installer compares, so it refuses everything.
	for _, pkg := range []struct {
		label, path string
		want        [4]bool
	}{
		{"signature value", path, [4]bool{true, false, false, false}},
		{"no hash", noHash, [4]bool{true, false, true, false}},
		{"content digest", digestHash, [4]bool{false, false, false, false}},
	} {
		t.Run(pkg.label, func(t *testing.T) {
			if checkInstall(t, pkg.path, files, pkg.label+": signed cabinets", pkg.want[0]) != pkg.want[0] {
				t.Error("the installed files are not the package's sources")
			}

			for i, tc := range []struct {
				label string
				data  func() []byte
			}{
				{"a changed byte in the second cabinet", func() []byte { return changed }},
				{"the second cabinet of another build by the same signer", func() []byte { return readFile(t, swapped) }},
				{"an unsigned second cabinet", func() []byte { return readFile(t, unsignedCab) }},
			} {
				if err := os.WriteFile(second, tc.data(), 0o600); err != nil {
					t.Fatal(err)
				}

				_ = checkInstall(t, pkg.path, files, pkg.label+": "+tc.label, pkg.want[i+1])

				if err := os.WriteFile(second, original, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}

	t.Run("untrusted", func(t *testing.T) {
		// A self-signed certificate that no store trusts installs: the
		// package pins the certificate in MsiDigitalCertificate, and the
		// package's own signature is what a user trusts.
		key := rsaLeafKey(t)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			Subject:      pkix.Name{CommonName: "msi-builder untrusted signer"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		}
		cert := newCertificate(t, tmpl, tmpl, key.Public(), key)

		untrustedPath, _ := signedProbe(t, files, sign.Options{Signer: key, Certificates: []*x509.Certificate{cert}})
		log := filepath.Join(t.TempDir(), "install.log")

		code := msiexecCode(t, log, "/i", untrustedPath)
		if code == 0 {
			msiexecQuiet(t, filepath.Join(t.TempDir(), "uninstall.log"), "/x", untrustedPath)
		} else {
			t.Errorf("msiexec exit %d for cabinets signed by the certificate the package lists", code)
		}

		t.Logf("SIG-09: cabinets signed by an untrusted self-signed certificate: msiexec exit %d\n%s", code, logLines(log))
	})
}

// TestICESignedCabinets runs the ICE suite, ICE81 among it, on a package
// with signed external cabinets.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICESignedCabinets(t *testing.T) {
	cub := iceCube(t)
	pki := newPKI(t, rsaLeafKey(t))
	path, _ := signedProbe(t, probeFilesOf('a'), pki.options(crypto.SHA256))

	requireCleanICE(t, path, cub)
}
