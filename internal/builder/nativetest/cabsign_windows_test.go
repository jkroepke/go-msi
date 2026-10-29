//go:build windows

package nativetest_test

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"errors"
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
	"github.com/jkroepke/msi-builder/pkg/sign"
)

var (
	pMsiGetFileSignatureInformationW = msiDLL.NewProc("MsiGetFileSignatureInformationW")
	pCertFreeCertificateContext      = syscall.NewLazyDLL("crypt32.dll").NewProc("CertFreeCertificateContext")
)

// fileSignatureHash returns the hash MsiGetFileSignatureInformation reports
// for the signed file at path, the value MsiDigitalSignature.Hash compares.
func fileSignatureHash(t *testing.T, path string) ([]byte, error) {
	t.Helper()

	var (
		cert uintptr
		hash = make([]byte, 64)
		size = uint32(len(hash))
	)

	r, _, _ := pMsiGetFileSignatureInformationW.Call(uintptr(unsafe.Pointer(wstr(t, path))), 0,
		uintptr(unsafe.Pointer(&cert)), uintptr(unsafe.Pointer(&hash[0])), uintptr(unsafe.Pointer(&size)))
	if cert != 0 {
		_, _, _ = pCertFreeCertificateContext.Call(cert)
	}

	if r != 0 {
		return nil, errors.New("MsiGetFileSignatureInformation: HRESULT 0x" + strings.ToUpper(hex.EncodeToString([]byte{byte(r >> 24), byte(r >> 16), byte(r >> 8), byte(r)})))
	}

	return hash[:size], nil
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

		digest, err := sign.SignCabinet(t.Context(), file, pki.options(algo))
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
			hash, err := fileSignatureHash(t, path)
			if err != nil {
				t.Errorf("%v: %v", algo, err)
			} else if !bytes.Equal(hash, digest) {
				t.Errorf("%v: MsiGetFileSignatureInformation hash %x, msi-builder digest %x", algo, hash, digest)
			}

			t.Logf("SIG-08: %v: WinVerifyTrust ok, MsiGetFileSignatureInformation hash %x, msi-builder digest %x", algo, hash, digest)
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

// expectRefused installs path, which must fail, and logs the error lines.
func expectRefused(t *testing.T, path, label string) {
	t.Helper()

	log := filepath.Join(t.TempDir(), "install.log")

	code := msiexecCode(t, log, "/i", path)
	if code == 0 {
		msiexecQuiet(t, filepath.Join(t.TempDir(), "uninstall.log"), "/x", path)
		t.Errorf("%s: the package installed", label)

		return
	}

	t.Logf("SIG-09: %s: msiexec exit %d\n%s", label, code, logLines(log))

	if _, err := os.Stat(filepath.Join(os.Getenv("ProgramFiles"), "Cabinet Probe")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s: the failed install left its folder: %v", label, err)
	}
}

// replaceFile copies src over dst.
func replaceFile(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestMsiexecChecksSignedCabinets installs a package whose three external
// cabinets are signed and listed in MsiDigitalSignature (SIG-09), then
// checks that Windows Installer refuses a changed cabinet, a cabinet signed
// by the same certificate for other content, and an unsigned cabinet, and
// records what it does for a certificate it does not trust.
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

	t.Run("signed", func(t *testing.T) {
		installProbe(t, path, files)
	})

	second := filepath.Join(dir, res.Cabinets[1].Name)

	original, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}

	restore := func() {
		if err := os.WriteFile(second, original, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("changed", func(t *testing.T) {
		defer restore()

		// The last byte before cbCabinet is cabinet data, not signature.
		changed := bytes.Clone(original)
		changed[binary.LittleEndian.Uint32(changed[8:])-1] ^= 1

		if err := os.WriteFile(second, changed, 0o600); err != nil {
			t.Fatal(err)
		}

		expectRefused(t, path, "a changed byte in the second cabinet")
	})

	t.Run("swapped", func(t *testing.T) {
		defer restore()

		// The same certificate signs other content in the same layout.
		otherPath, other := signedProbe(t, probeFilesOf('x'), pki.options(crypto.SHA256))
		replaceFile(t, filepath.Join(filepath.Dir(otherPath), other.Cabinets[1].Name), second)

		expectRefused(t, path, "the second cabinet of another build by the same signer")
	})

	t.Run("unsigned", func(t *testing.T) {
		defer restore()

		unsignedPath, unsigned := buildProbe(t, "  compression: mszip\n  embedCabinets: false", files,
			builder.Options{BuildTime: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC), CabinetSize: 1})
		replaceFile(t, filepath.Join(filepath.Dir(unsignedPath), unsigned.Cabinets[1].Name), second)

		expectRefused(t, path, "an unsigned second cabinet")
	})

	t.Run("untrusted", func(t *testing.T) {
		// A self-signed certificate that no store trusts: recorded, not
		// asserted.
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
