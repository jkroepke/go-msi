package sign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/cab"
	"github.com/jkroepke/msi-builder/internal/spool"
)

// reservedCabinet is a two-file MSZIP cabinet with a signature reserve.
func reservedCabinet(t *testing.T, reserve bool) *spool.Buffer {
	t.Helper()

	raw, err := cab.Write([]cab.File{
		{Name: "a.txt", Data: []byte("signed cabinet")},
		{Name: "b.bin", Data: bytes.Repeat([]byte("msi"), 40000)},
	}, cab.Options{Compression: cab.MSZIP, SignatureReserve: reserve})
	if err != nil {
		t.Fatal(err)
	}

	buf := &spool.Buffer{}
	if _, err := buf.Write(raw); err != nil {
		t.Fatal(err)
	}

	return buf
}

func TestSignCabinet(t *testing.T) {
	t.Parallel()

	for name, keys := range map[string]testKey{"rsa": rsaKey(t), "ecdsa": ecKey(t)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			buf := reservedCabinet(t, true)
			size := buf.Len()

			if _, err := VerifyCabinet(buf); !errors.Is(err, ErrCabinetNotSigned) {
				t.Fatalf("VerifyCabinet of the unsigned cabinet = %v", err)
			}

			before, err := CabinetDigest(buf, crypto.SHA256)
			if err != nil {
				t.Fatal(err)
			}

			digest, err := SignCabinet(context.Background(), buf, Options{Signer: keys.signer, Certificates: keys.certs})
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(digest, before) {
				t.Error("SignCabinet returned another digest than CabinetDigest")
			}

			signed := buf.Bytes()
			sigSize := int64(binary.LittleEndian.Uint32(signed[cabSigHeader+8:]))

			if buf.Len() != size+sigSize || sigSize%cabSigAlign != 0 {
				t.Errorf("%d bytes with a signature of %d, want %d plus the signature, padded to 8", buf.Len(), sigSize, size)
			}

			if got := int64(binary.LittleEndian.Uint32(signed[8:])); got != size {
				t.Errorf("cbCabinet %d, want %d: the signature is not part of the cabinet", got, size)
			}

			sig, err := VerifyCabinet(buf)
			if err != nil {
				t.Fatal(err)
			}

			if !sig.Signer.Equal(keys.certs[0]) || !bytes.Equal(sig.Digest, digest) || sig.Hash != crypto.SHA256 {
				t.Errorf("signature %+v", sig)
			}

			after, err := CabinetDigest(buf, crypto.SHA256)
			if err != nil || !bytes.Equal(after, digest) {
				t.Errorf("CabinetDigest of the signed cabinet = %x, %v; want the signed digest", after, err)
			}

			if files, err := cab.Read(signed); err != nil || len(files) != 2 {
				t.Errorf("the signed cabinet reads as %d files: %v", len(files), err)
			}

			if _, err := SignCabinet(context.Background(), buf, Options{Signer: keys.signer, Certificates: keys.certs}); err == nil {
				t.Error("a signed cabinet was signed again")
			}
		})
	}
}

// TestVerifyCabinetDetectsTampering flips bytes the digest covers and bytes
// it leaves out.
func TestVerifyCabinetDetectsTampering(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)
	buf := reservedCabinet(t, true)

	if _, err := SignCabinet(context.Background(), buf, Options{Signer: keys.signer, Certificates: keys.certs}); err != nil {
		t.Fatal(err)
	}

	signed := buf.Bytes()
	size := int64(binary.LittleEndian.Uint32(signed[8:]))

	for _, tc := range []struct {
		name  string
		off   int64
		valid bool
	}{
		{"header reserved field", 0x04, true},
		{"iCabinet", 0x22, true},
		{"setID", 0x20, false},
		{"folder record", cabFolders, false},
		{"file record", cabFolders + cabFolderSize + 4, false},
		{"last data byte", size - 1, false},
		{"signature", size + 100, false},
	} {
		tampered := bytes.Clone(signed)
		tampered[tc.off] ^= 1

		_, err := VerifyCabinet(bytes.NewReader(tampered))
		if tc.valid && err != nil || !tc.valid && err == nil {
			t.Errorf("%s: VerifyCabinet = %v", tc.name, err)
		}
	}
}

func TestSignCabinetRejects(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)
	opts := Options{Signer: keys.signer, Certificates: keys.certs}

	if _, err := SignCabinet(context.Background(), reservedCabinet(t, false), opts); err == nil {
		t.Error("a cabinet without a signature reserve was signed")
	}

	if _, err := SignCabinet(context.Background(), reservedCabinet(t, true), Options{Signer: keys.signer}); err == nil {
		t.Error("a signer without a certificate was accepted")
	}

	short := reservedCabinet(t, true)
	if _, err := VerifyCabinet(bytes.NewReader(short.Bytes()[:30])); err == nil {
		t.Error("a truncated cabinet was accepted")
	}
}

func TestSignCabinetTimestamp(t *testing.T) {
	t.Parallel()

	keys := rsaKey(t)
	authority := &tsa{key: testKey{keys.signer, []*x509.Certificate{newCert(t, keys.signer, "test TSA", x509.ExtKeyUsageTimeStamping)}}, time: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}

	srv := httptest.NewServer(authority)
	t.Cleanup(srv.Close)

	buf := reservedCabinet(t, true)

	if _, err := SignCabinet(context.Background(), buf, Options{Signer: keys.signer, Certificates: keys.certs, TimestampURL: srv.URL, HTTPClient: srv.Client()}); err != nil {
		t.Fatal(err)
	}

	sig, err := VerifyCabinet(buf)
	if err != nil {
		t.Fatal(err)
	}

	if !sig.Timestamp.Equal(authority.time) {
		t.Errorf("Timestamp = %v, want %v", sig.Timestamp, authority.time)
	}
}
