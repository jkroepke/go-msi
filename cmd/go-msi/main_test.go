package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"

	"github.com/jkroepke/go-msi/internal/testpe"
)

func TestBuildValidateInspect(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	src, err := os.ReadFile(filepath.Join("..", "..", "examples", "minimal", "msi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	yaml := filepath.Join(dir, "msi.yaml")
	if err := os.WriteFile(yaml, src, 0o600); err != nil {
		t.Fatal(err)
	}

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	if err := os.WriteFile(filepath.Join(dir, "hello.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	if code := run([]string{"validate", yaml}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate: exit %d: %s", code, stderr.String())
	}

	out := filepath.Join(dir, "out.msi")

	if code := run([]string{"build", "-o", out, yaml}, &stdout, &stderr); code != 0 {
		t.Fatalf("build: exit %d: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "1 files") {
		t.Errorf("build output %q", stdout.String())
	}

	stdout.Reset()

	if code := run([]string{"inspect", "--table", "File", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("inspect: exit %d: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "hello.exe") {
		t.Errorf("inspect output %q", stdout.String())
	}

	stdout.Reset()

	if code := run([]string{"inspect", out}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "summary  9:") {
		t.Errorf("inspect: exit %d, output %q", code, stdout.String())
	}

	if code := run([]string{"build", filepath.Join(dir, "missing.yaml")}, &stdout, &stderr); code != 1 {
		t.Errorf("missing file: exit %d", code)
	}

	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Errorf("no command: exit %d", code)
	}

	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Errorf("version: exit %d", code)
	}
}

// testKeyFiles are the paths writeTestKey wrote.
type testKeyFiles struct{ cert, key, pfx string }

func writeTestKey(t *testing.T, dir string) testKeyFiles {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "go-msi CLI test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
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

	pfx, err := pkcs12.Modern.Encode(key, cert, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	files := testKeyFiles{cert: filepath.Join(dir, "cert.pem"), key: filepath.Join(dir, "key.pem"), pfx: filepath.Join(dir, "key.pfx")}

	for name, data := range map[string][]byte{
		files.cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		files.key:  pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
		files.pfx:  pfx,
	} {
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return files
}

func TestSignVerify(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	src, err := os.ReadFile(filepath.Join("..", "..", "examples", "minimal", "msi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	yaml := filepath.Join(dir, "msi.yaml")
	if err := os.WriteFile(yaml, src, 0o600); err != nil {
		t.Fatal(err)
	}

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	if err := os.WriteFile(filepath.Join(dir, "hello.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	keys := writeTestKey(t, dir)
	certFile, keyFile, pfxFile := keys.cert, keys.key, keys.pfx

	var stdout, stderr bytes.Buffer

	unsigned := filepath.Join(dir, "unsigned.msi")
	if code := run([]string{"build", "-q", "-o", unsigned, yaml}, &stdout, &stderr); code != 0 {
		t.Fatalf("build: exit %d: %s", code, stderr.String())
	}

	if code := run([]string{"verify", unsigned}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "not signed") {
		t.Errorf("verify unsigned: exit %d: %s", code, stderr.String())
	}

	signed := filepath.Join(dir, "signed.msi")
	if code := run([]string{"build", "-q", "-o", signed, "--cert", certFile, "--key", keyFile, yaml}, &stdout, &stderr); code != 0 {
		t.Fatalf("build --cert: exit %d: %s", code, stderr.String())
	}

	stdout.Reset()

	if code := run([]string{"verify", signed}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "signature valid (basic, SHA-256, no timestamp)") {
		t.Errorf("verify: exit %d: %s %s", code, stdout.String(), stderr.String())
	}

	resigned := filepath.Join(dir, "resigned.msi")
	if code := run([]string{"sign", "--pfx", pfxFile, "--hash", "sha384", "-o", resigned, unsigned}, &stdout, &stderr); code != 0 {
		t.Fatalf("sign: exit %d: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "SHA-384") {
		t.Errorf("sign output %q", stdout.String())
	}

	for name, args := range map[string][]string{
		"no key":       {"sign", unsigned},
		"both":         {"sign", "--pfx", pfxFile, "--cert", certFile, "--key", keyFile, unsigned},
		"bad hash":     {"sign", "--pfx", pfxFile, "--hash", "md5", unsigned},
		"missing env":  {"sign", "--pfx", pfxFile, "--password-env", "GO_MSI_TEST_UNSET_VARIABLE", unsigned},
		"cert only":    {"sign", "--cert", certFile, unsigned},
		"no key build": {"build", "--timestamp-url", "http://example.com", yaml},
		"env with pem": {"sign", "--cert", certFile, "--key", keyFile, "--password-env", "HOME", unsigned},
		"no package":   {"verify"},
		"missing file": {"verify", filepath.Join(dir, "missing.msi")},
	} {
		if code := run(args, &stdout, &stderr); code == 0 {
			t.Errorf("%s: exit 0", name)
		}
	}
}

func TestFlagErrorsGoToStderr(t *testing.T) {
	t.Parallel()

	for _, cmd := range []string{"build", "validate", "inspect", "sign", "verify"} {
		var stdout, stderr bytes.Buffer

		if code := run([]string{cmd, "--no-such-flag"}, &stdout, &stderr); code == 0 {
			t.Errorf("%s: exit 0 for an unknown flag", cmd)
		}

		// "Usage of" is the FlagSet's own output; run adds the error line.
		if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Usage of "+cmd) {
			t.Errorf("%s: stdout %q, stderr %q", cmd, stdout.String(), stderr.String())
		}
	}
}
