package sign

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"

	"software.sslmate.com/src/go-pkcs12"
)

// LoadPEM reads a certificate chain (signing certificate first) and a
// private key from PEM data. The key is PKCS #8, PKCS #1 RSA or SEC 1 EC,
// unencrypted. Both may come from the same data.
func LoadPEM(certPEM, keyPEM []byte) (crypto.Signer, []*x509.Certificate, error) {
	var certs []*x509.Certificate

	for _, block := range pemBlocks(certPEM, "CERTIFICATE") {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("sign: certificate: %w", err)
		}

		certs = append(certs, cert)
	}

	if len(certs) == 0 {
		return nil, nil, errors.New("sign: no CERTIFICATE block found")
	}

	blocks := pemBlocks(keyPEM, "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "ENCRYPTED PRIVATE KEY")
	if len(blocks) == 0 {
		return nil, nil, errors.New("sign: no private key block found")
	}

	signer, err := parseKey(blocks[0])
	if err != nil {
		return nil, nil, err
	}

	return signer, orderChain(signer, certs), nil
}

// pemBlocks returns the blocks of data with one of the given types.
func pemBlocks(data []byte, types ...string) []*pem.Block {
	var out []*pem.Block

	for rest := data; ; {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			return out
		}

		if slices.Contains(types, block.Type) {
			out = append(out, block)
		}
	}
}

func parseKey(block *pem.Block) (crypto.Signer, error) {
	var (
		key any
		err error
	)

	switch block.Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, errors.New("sign: encrypted PEM keys are not supported; use a PKCS #12 file")
	}

	if err != nil {
		return nil, fmt.Errorf("sign: private key: %w", err)
	}

	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("sign: unsupported key type %T", key)
	}

	return signer, nil
}

// LoadPKCS12 reads a key and its certificate chain from a .pfx/.p12 file.
func LoadPKCS12(data []byte, password string) (crypto.Signer, []*x509.Certificate, error) {
	key, cert, chain, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return nil, nil, fmt.Errorf("sign: pkcs12: %w", err)
	}

	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("sign: unsupported key type %T", key)
	}

	return signer, append([]*x509.Certificate{cert}, chain...), nil
}

// orderChain moves the certificate matching the key to the front.
func orderChain(signer crypto.Signer, certs []*x509.Certificate) []*x509.Certificate {
	type equaler interface{ Equal(x crypto.PublicKey) bool }

	pub, ok := signer.Public().(equaler)
	if !ok {
		return certs
	}

	for i, c := range certs {
		if pub.Equal(c.PublicKey) {
			out := append([]*x509.Certificate{c}, certs[:i]...)

			return append(out, certs[i+1:]...)
		}
	}

	return certs
}
