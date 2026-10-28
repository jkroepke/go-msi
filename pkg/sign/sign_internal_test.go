package sign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"

	"github.com/jkroepke/msi-builder/pkg/cfb"
)

// vectorStreams is the chapter 01 compound-file vector.
func vectorStreams() []cfb.Stream {
	large := make([]byte, 5000)
	for i := range large {
		large[i] = byte(i)
	}

	return []cfb.Stream{
		{Name: "Large", Data: large},
		{Name: "Small", Data: []byte(strings.Repeat("0123456789", 10))},
	}
}

func vectorPackage(t *testing.T, extra ...cfb.Stream) []byte {
	t.Helper()

	out, err := cfb.Write(append(vectorStreams(), extra...), cfb.Options{Version: 3, CLSID: cfb.CLSIDMSI})
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func readFile(t *testing.T, pkg []byte) *cfb.File {
	t.Helper()

	file, err := cfb.Read(pkg)
	if err != nil {
		t.Fatal(err)
	}

	return file
}

// TestDigestVectors checks the MSISPEC §12 worked example.
func TestDigestVectors(t *testing.T) {
	t.Parallel()

	file := readFile(t, vectorPackage(t))

	digest, err := contentDigest(file, crypto.SHA256, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(digest); got != "7a45b1cc8e2cf7cdb5c2e3d5e6ce141e6b0d42b4adaf1b79daf55def67ee3653" {
		t.Errorf("D = %s", got)
	}

	pre, err := preHash(file, crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(pre); got != "88116bedc271b4e0dbdf9e2dc6a14ed8ca7395a2389940f515eda8810aed53b6" {
		t.Errorf("P = %s", got)
	}

	ext := readFile(t, vectorPackage(t, cfb.Stream{Name: StreamSignatureEx, Data: pre}))

	dExt, err := contentDigest(ext, crypto.SHA256, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(dExt); got != "3bce8833b4f6d3abebe06385f7a8e42a48ac4732f3b88b96109d57ef6a344e7e" {
		t.Errorf("D_ext = %s", got)
	}

	// Digest ignores the Ex stream, as a basic-mode signature would.
	basic, err := Digest(vectorPackage(t, cfb.Stream{Name: StreamSignatureEx, Data: pre}), crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(basic, digest) {
		t.Errorf("Digest with an Ex stream = %x, want D", basic)
	}

	content, err := indirectData(crypto.SHA256, digest)
	if err != nil {
		t.Fatal(err)
	}

	body, err := contents(content)
	if err != nil {
		t.Fatal(err)
	}

	// 105 bytes of DER with D at 0x49; messageDigest covers the body.
	if len(content) != 105 || !bytes.Equal(content[0x49:0x69], digest) {
		t.Fatalf("SpcIndirectDataContent is %d bytes: %x", len(content), content)
	}

	if sum := sha256.Sum256(body); hex.EncodeToString(sum[:]) != "698288472286219a09b436458cc3ce51b4ab37c88a3042e67ac67d7e638a985a" {
		t.Errorf("messageDigest = %x", sum)
	}
}

func TestSigOrder(t *testing.T) {
	t.Parallel()

	names := []string{"\u4840\u3f3f", "Small", "\u0005SummaryInformation", "A", "AB", "\u0102x"}
	ents := make([]*cfb.Entry, 0, len(names))

	for _, n := range names {
		ents = append(ents, &cfb.Entry{Name: n})
	}

	sorted := make([]string, 0, len(ents))

	slices.SortFunc(ents, sigOrder)

	for _, e := range ents {
		sorted = append(sorted, e.Name)
	}

	// UTF-16LE bytes: 02 01 < 05 00 < 40 48 < 41 00 ('A') < "AB" < 53 00.
	want := []string{"\u0102x", "\u0005SummaryInformation", "\u4840\u3f3f", "A", "AB", "Small"}
	if strings.Join(sorted, ",") != strings.Join(want, ",") {
		t.Errorf("SigOrder = %q, want %q", sorted, want)
	}
}

type testKey struct {
	signer crypto.Signer
	certs  []*x509.Certificate
}

// newCert returns a self-signed certificate for key.
func newCert(t *testing.T, key crypto.Signer, cn string, eku x509.ExtKeyUsage) *x509.Certificate {
	t.Helper()

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{eku},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return cert
}

func rsaKey(t *testing.T) testKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	return testKey{key, []*x509.Certificate{newCert(t, key, "msi-builder test", x509.ExtKeyUsageCodeSigning)}}
}

func ecKey(t *testing.T) testKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	return testKey{key, []*x509.Certificate{newCert(t, key, "msi-builder ec test", x509.ExtKeyUsageCodeSigning)}}
}

func TestSignVerify(t *testing.T) {
	t.Parallel()

	rsaKeys := rsaKey(t)
	ecKeys := ecKey(t)

	cases := []struct {
		name string
		key  testKey
		hash crypto.Hash
	}{
		{"rsa-sha256", rsaKeys, 0},
		{"rsa-sha1", rsaKeys, crypto.SHA1},
		{"rsa-sha384", rsaKeys, crypto.SHA384},
		{"rsa-sha512", rsaKeys, crypto.SHA512},
		{"ecdsa-sha256", ecKeys, crypto.SHA256},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pkg := vectorPackage(t)
			opts := Options{Signer: tc.key.signer, Certificates: tc.key.certs, Hash: tc.hash, Description: "Test Product", URL: "https://example.com"}

			signed, err := Sign(context.Background(), pkg, opts)
			if err != nil {
				t.Fatal(err)
			}

			sig, err := Verify(signed)
			if err != nil {
				t.Fatal(err)
			}

			if sig.Signer.SerialNumber.Cmp(tc.key.certs[0].SerialNumber) != 0 || sig.Extended || !sig.Timestamp.IsZero() {
				t.Errorf("unexpected signature %+v", sig)
			}

			// Re-signing replaces the signature instead of adding one.
			again, err := Sign(context.Background(), signed, opts)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := Verify(again); err != nil {
				t.Fatal(err)
			}

			unsigned, err := Unsign(signed)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := Verify(unsigned); !errors.Is(err, ErrNotSigned) {
				t.Errorf("Verify(unsigned) = %v", err)
			}
		})
	}
}

func TestSignDeterministic(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)
	opts := Options{Signer: keys.signer, Certificates: keys.certs}

	first, err := Sign(context.Background(), vectorPackage(t), opts)
	if err != nil {
		t.Fatal(err)
	}

	second, err := Sign(context.Background(), vectorPackage(t), opts)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(first, second) {
		t.Error("two RSA signatures of the same package differ")
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)

	signed, err := Sign(context.Background(), vectorPackage(t), Options{Signer: keys.signer, Certificates: keys.certs})
	if err != nil {
		t.Fatal(err)
	}

	file := readFile(t, signed)

	streams := file.Streams()
	for i := range streams {
		streams[i].Data = bytes.Clone(streams[i].Data)
	}

	t.Run("content", func(t *testing.T) {
		t.Parallel()

		changed := cloneStreams(streams)
		changed[0].Data[10] ^= 1

		out, err := cfb.Write(changed, cfb.Options{Version: 3, CLSID: cfb.CLSIDMSI})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := Verify(out); err == nil || !strings.Contains(err.Error(), "content does not match") {
			t.Errorf("Verify = %v", err)
		}
	})

	t.Run("clsid", func(t *testing.T) {
		t.Parallel()

		clsid := cfb.CLSIDMSI
		clsid[0] ^= 1

		out, err := cfb.Write(cloneStreams(streams), cfb.Options{Version: 3, CLSID: clsid})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := Verify(out); err == nil {
			t.Error("a changed root CLSID still verifies")
		}
	})

	t.Run("relayout", func(t *testing.T) {
		t.Parallel()

		// A different container version keeps a basic-mode signature valid.
		out, err := cfb.Write(cloneStreams(streams), cfb.Options{Version: 4, CLSID: cfb.CLSIDMSI})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := Verify(out); err != nil {
			t.Errorf("Verify after re-layout = %v", err)
		}
	})

	t.Run("signature", func(t *testing.T) {
		t.Parallel()

		changed := cloneStreams(streams)
		for i := range changed {
			if changed[i].Name == StreamSignature {
				changed[i].Data[len(changed[i].Data)-5] ^= 1
			}
		}

		out, err := cfb.Write(changed, cfb.Options{Version: 3, CLSID: cfb.CLSIDMSI})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := Verify(out); err == nil {
			t.Error("a damaged signature still verifies")
		}
	})
}

func cloneStreams(in []cfb.Stream) []cfb.Stream {
	out := make([]cfb.Stream, len(in))
	for i, s := range in {
		out[i] = cfb.Stream{Name: s.Name, Data: bytes.Clone(s.Data)}
	}

	return out
}

func TestSignRejects(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)
	other := rsaKey(t)

	small, err := rsa.GenerateKey(rand.Reader, 1024) //nolint:gosec // checks that small keys are refused
	if err != nil {
		t.Fatal(err)
	}

	smallCert := newCert(t, small, "small", x509.ExtKeyUsageCodeSigning)

	cases := map[string]Options{
		"no key":       {},
		"mismatch":     {Signer: keys.signer, Certificates: other.certs},
		"small key":    {Signer: small, Certificates: []*x509.Certificate{smallCert}},
		"bad hash":     {Signer: keys.signer, Certificates: keys.certs, Hash: crypto.MD5},
		"bad tsa":      {Signer: keys.signer, Certificates: keys.certs, TimestampURL: "http://127.0.0.1:1/"},
		"not a server": {Signer: keys.signer, Certificates: keys.certs, TimestampURL: "://"},
	}

	for name, opts := range cases {
		if _, err := Sign(context.Background(), vectorPackage(t), opts); err == nil {
			t.Errorf("%s: Sign succeeded", name)
		}
	}

	if _, err := Sign(context.Background(), []byte("not a package"), Options{Signer: keys.signer, Certificates: keys.certs}); err == nil {
		t.Error("Sign accepted garbage")
	}
}

func TestDigestAttach(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)
	pkg := vectorPackage(t)

	digest, err := Digest(pkg, crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}

	// An external signer builds the PKCS #7 from D.
	pkcs7DER, err := buildSignature(context.Background(), digest, &Options{Signer: keys.signer, Certificates: keys.certs})
	if err != nil {
		t.Fatal(err)
	}

	out, sig, err := Attach(pkg, pkcs7DER)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(sig.Digest, digest) {
		t.Error("Attach verified a different digest")
	}

	if _, err := Verify(out); err != nil {
		t.Fatal(err)
	}

	// A signature for other content is refused.
	other := vectorPackage(t, cfb.Stream{Name: "Extra", Data: []byte("x")})
	if _, _, err := Attach(other, pkcs7DER); err == nil {
		t.Error("Attach accepted a signature over other content")
	}
}

// tsa is a minimal RFC 3161 server for the tests.
type tsa struct {
	key  testKey
	time time.Time
	// tamper changes the token's imprint.
	tamper bool
	// dropNonce leaves the nonce out.
	dropNonce bool
}

func (s *tsa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	var req struct {
		Version        int
		MessageImprint messageImprint
		Nonce          *big.Int `asn1:"optional"`
		CertReq        bool     `asn1:"optional"`
	}

	if _, err := asn1.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	imprint := req.MessageImprint
	if s.tamper {
		imprint.HashedMessage = bytes.Clone(imprint.HashedMessage)
		imprint.HashedMessage[0] ^= 1
	}

	parts := [][]byte{
		mustMarshal(1),
		mustMarshal(asn1.ObjectIdentifier{1, 2, 3, 4}),
		mustMarshal(imprint),
		mustMarshal(big.NewInt(42)),
		mustMarshal(asn1.RawValue{Tag: asn1.TagGeneralizedTime, Bytes: []byte(s.time.UTC().Format("20060102150405.000Z"))}),
	}

	if !s.dropNonce && req.Nonce != nil {
		parts = append(parts, mustMarshal(req.Nonce))
	}

	info := sequence(parts...)
	econtent := mustMarshal(info)

	sum := sha256.Sum256(info)
	attrs := [][]byte{
		newAttribute(oidContentType, mustMarshal(oidTSTInfo)),
		newAttribute(oidMessageDigest, mustMarshal(sum[:])),
	}
	signedAttrs := marshalSet(0, asn1.ClassContextSpecific, attrs)
	setBody, _ := contents(signedAttrs)
	digest := sha256.Sum256(retagSet(setBody))

	sig, err := s.key.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	cert := s.key.certs[0]
	alg, _ := digestAlgorithm(crypto.SHA256)
	signer := sequence(
		mustMarshal(1),
		sequence(cert.RawIssuer, mustMarshal(cert.SerialNumber)),
		mustMarshal(alg),
		signedAttrs,
		sequence(mustMarshal(oidRSAEncryption), mustMarshal(asn1Null)),
		mustMarshal(sig),
	)
	signedDER := sequence(
		mustMarshal(3),
		marshalSet(asn1.TagSet, asn1.ClassUniversal, [][]byte{mustMarshal(alg)}),
		sequence(mustMarshal(oidTSTInfo), tlv(asn1.ClassContextSpecific, 0, true, econtent)),
		marshalSet(0, asn1.ClassContextSpecific, [][]byte{cert.Raw}),
		marshalSet(asn1.TagSet, asn1.ClassUniversal, [][]byte{signer}),
	)
	token := sequence(mustMarshal(oidSignedData), tlv(asn1.ClassContextSpecific, 0, true, signedDER))

	w.Header().Set("Content-Type", "application/timestamp-reply")
	_, _ = w.Write(sequence(sequence(mustMarshal(0)), token))
}

func TestTimestamp(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)
	authority := &tsa{key: testKey{keys.signer, []*x509.Certificate{newCert(t, keys.signer, "test TSA", x509.ExtKeyUsageTimeStamping)}}, time: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)}

	srv := httptest.NewServer(authority)
	t.Cleanup(srv.Close)

	opts := Options{Signer: keys.signer, Certificates: keys.certs, TimestampURL: srv.URL, HTTPClient: srv.Client()}

	signed, err := Sign(context.Background(), vectorPackage(t), opts)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := Verify(signed)
	if err != nil {
		t.Fatal(err)
	}

	if !sig.Timestamp.Equal(authority.time) {
		t.Errorf("Timestamp = %v, want %v", sig.Timestamp, authority.time)
	}

	for name, bad := range map[string]*tsa{
		"tampered": {key: authority.key, time: authority.time, tamper: true},
		"no nonce": {key: authority.key, time: authority.time, dropNonce: true},
	} {
		srv := httptest.NewServer(bad)

		opts.TimestampURL = srv.URL
		if _, err := Sign(context.Background(), vectorPackage(t), opts); err == nil {
			t.Errorf("%s: Sign accepted the token", name)
		}

		srv.Close()
	}
}

func TestParseTimestampResponse(t *testing.T) {
	t.Parallel()

	if _, err := parseTimestampResponse(sequence(sequence(mustMarshal(2)))); err == nil || !strings.Contains(err.Error(), "status 2") {
		t.Errorf("rejection: %v", err)
	}

	if _, err := parseTimestampResponse(sequence(sequence(mustMarshal(0)))); err == nil {
		t.Error("granted without a token was accepted")
	}

	if _, err := parseTimestampResponse([]byte{1, 2, 3}); err == nil {
		t.Error("garbage was accepted")
	}
}

func TestLoadKeys(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)

	rsaPriv, ok := keys.signer.(*rsa.PrivateKey)
	if !ok {
		t.Fatal("not an RSA key")
	}

	pkcs8, err := x509.MarshalPKCS8PrivateKey(rsaPriv)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: keys.certs[0].Raw})

	for name, keyPEM := range map[string][]byte{
		"pkcs8": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}),
		"pkcs1": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaPriv)}),
	} {
		signer, certs, err := LoadPEM(certPEM, keyPEM)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if err := checkKey(signer, certs[0]); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	ecKeys := ecKey(t)

	ecPriv, ok := ecKeys.signer.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatal("not an ECDSA key")
	}

	sec1, err := x509.MarshalECPrivateKey(ecPriv)
	if err != nil {
		t.Fatal(err)
	}

	// The key's certificate is found even when it is not first.
	both := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: keys.certs[0].Raw}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ecKeys.certs[0].Raw})...)

	signer, certs, err := LoadPEM(both, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}))
	if err != nil {
		t.Fatal(err)
	}

	if err := checkKey(signer, certs[0]); err != nil || len(certs) != 2 {
		t.Errorf("EC chain: %v, %d certs", err, len(certs))
	}

	if _, _, err := LoadPEM(certPEM, []byte("nothing")); err == nil {
		t.Error("missing key was accepted")
	}

	if _, _, err := LoadPEM([]byte("nothing"), nil); err == nil {
		t.Error("missing certificate was accepted")
	}

	if _, _, err := LoadPEM(certPEM, pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte{0}})); err == nil {
		t.Error("encrypted key was accepted")
	}

	pfx, err := pkcs12.Modern.Encode(rsaPriv, keys.certs[0], nil, "secret")
	if err != nil {
		t.Fatal(err)
	}

	signer, certs, err = LoadPKCS12(pfx, "secret")
	if err != nil {
		t.Fatal(err)
	}

	if err := checkKey(signer, certs[0]); err != nil {
		t.Error(err)
	}

	if _, _, err := LoadPKCS12(pfx, "wrong"); err == nil {
		t.Error("wrong password was accepted")
	}
}
