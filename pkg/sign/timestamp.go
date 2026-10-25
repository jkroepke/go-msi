package sign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"
)

// RFC 3161 structures.
type messageImprint struct {
	HashAlgorithm pkix.AlgorithmIdentifier
	HashedMessage []byte
}

type timeStampResp struct {
	Status         asn1.RawValue
	TimeStampToken asn1.RawValue `asn1:"optional"`
}

// tstInfo is the part of TSTInfo that Sign and Verify use.
type tstInfo struct {
	GenTime        time.Time
	Nonce          *big.Int
	MessageImprint messageImprint
}

// accuracy is only parsed to step over it; a RawValue would swallow the
// nonce when accuracy is absent.
type accuracy struct {
	Seconds int `asn1:"optional"`
	Millis  int `asn1:"optional,tag:0"`
	Micros  int `asn1:"optional,tag:1"`
}

// ErrTimestampUnavailable wraps failures to reach the time-stamping
// authority or a non-200 answer, as opposed to a bad reply.
var ErrTimestampUnavailable = errors.New("sign: timestamp authority unavailable")

// defaultClient is used without Options.HTTPClient; a stalled authority
// must not hang a build.
var defaultClient = &http.Client{Timeout: time.Minute} //nolint:gochecknoglobals // shared client

// maxResponse caps the timestamp reply.
const maxResponse = 1 << 20

// timestamp asks the TSA in opts for a token over the encrypted digest sig
// and returns the DER TimeStampToken.
func timestamp(ctx context.Context, opts *Options, algo crypto.Hash, sig []byte) ([]byte, error) {
	imprint, err := hashOf(algo, sig)
	if err != nil {
		return nil, err
	}

	alg, err := digestAlgorithm(algo)
	if err != nil {
		return nil, err
	}

	random := opts.Rand
	if random == nil {
		random = rand.Reader
	}

	nonce, err := rand.Int(random, new(big.Int).Lsh(big.NewInt(1), 63))
	if err != nil {
		return nil, fmt.Errorf("sign: timestamp nonce: %w", err)
	}

	body, err := postTimestampRequest(ctx, opts, sequence(
		mustMarshal(1),
		mustMarshal(messageImprint{HashAlgorithm: alg, HashedMessage: imprint}),
		mustMarshal(nonce),
		mustMarshal(true),
	))
	if err != nil {
		return nil, err
	}

	token, err := parseTimestampResponse(body)
	if err != nil {
		return nil, err
	}

	info, err := verifyToken(token, sig)
	if err != nil {
		return nil, err
	}

	if info.Nonce == nil || info.Nonce.Cmp(nonce) != 0 {
		return nil, errors.New("sign: timestamp: the reply does not carry our nonce")
	}

	return token, nil
}

// postTimestampRequest sends a DER TimeStampReq and returns the reply body.
func postTimestampRequest(ctx context.Context, opts *Options, req []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.TimestampURL, bytes.NewReader(req))
	if err != nil {
		return nil, fmt.Errorf("sign: timestamp: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/timestamp-query")
	httpReq.Header.Set("Accept", "application/timestamp-reply")

	client := opts.HTTPClient
	if client == nil {
		client = defaultClient
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTimestampUnavailable, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s answered %s", ErrTimestampUnavailable, opts.TimestampURL, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("sign: timestamp: %w", err)
	}

	if len(body) > maxResponse {
		return nil, errors.New("sign: timestamp: reply too large")
	}

	return body, nil
}

// parseTimestampResponse returns the token of a granted TimeStampResp.
func parseTimestampResponse(body []byte) ([]byte, error) {
	var resp timeStampResp

	if err := unmarshalExact(body, &resp); err != nil {
		return nil, fmt.Errorf("sign: timestamp: malformed reply: %w", err)
	}

	// PKIStatusInfo is { status, statusString?, failInfo? }; read the
	// status integer first.
	var status int
	if _, err := asn1.Unmarshal(resp.Status.Bytes, &status); err != nil {
		return nil, fmt.Errorf("sign: timestamp: malformed status: %w", err)
	}

	if status != 0 && status != 1 {
		return nil, fmt.Errorf("sign: timestamp: request refused with status %d", status)
	}

	if len(resp.TimeStampToken.FullBytes) == 0 {
		return nil, errors.New("sign: timestamp: granted reply without a token")
	}

	return resp.TimeStampToken.FullBytes, nil
}

// verifyToken checks the token's own signature and that its message
// imprint is the hash of message, in whatever algorithm the token names.
func verifyToken(token, message []byte) (*tstInfo, error) {
	sdata, err := parseSignedData(token)
	if err != nil {
		return nil, fmt.Errorf("sign: timestamp token: %w", err)
	}

	if !sdata.contentType.Equal(oidTSTInfo) {
		return nil, fmt.Errorf("sign: timestamp token holds %v, not TSTInfo", sdata.contentType)
	}

	if _, _, err := sdata.verify(); err != nil {
		return nil, fmt.Errorf("sign: timestamp token: %w", err)
	}

	info, err := parseTSTInfo(sdata.content)
	if err != nil {
		return nil, err
	}

	algo, err := hashFromOID(info.MessageImprint.HashAlgorithm.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("sign: timestamp token: %w", err)
	}

	want, err := hashOf(algo, message)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(info.MessageImprint.HashedMessage, want) {
		return nil, errors.New("sign: timestamp token does not cover this signature")
	}

	return info, nil
}

// parseTSTInfo decodes the TSTInfo inside the token's eContent octets.
func parseTSTInfo(content []byte) (*tstInfo, error) {
	var raw struct { //nolint:govet // field order is the ASN.1 order
		Version        int
		Policy         asn1.ObjectIdentifier
		MessageImprint messageImprint
		SerialNumber   *big.Int
		GenTime        asn1.RawValue
		Accuracy       accuracy      `asn1:"optional"`
		Ordering       bool          `asn1:"optional"`
		Nonce          *big.Int      `asn1:"optional"`
		TSA            asn1.RawValue `asn1:"optional,explicit,tag:0"`
		Extensions     asn1.RawValue `asn1:"optional,tag:1"`
	}

	if err := unmarshalExact(content, &raw); err != nil {
		return nil, fmt.Errorf("sign: timestamp token: malformed TSTInfo: %w", err)
	}

	// Parsed by hand: Go's DER parser refuses fractions with trailing
	// zeros, which some authorities send.
	genTime, err := time.Parse("20060102150405.999999999Z", string(raw.GenTime.Bytes))
	if raw.GenTime.Tag != asn1.TagGeneralizedTime || err != nil {
		return nil, fmt.Errorf("sign: timestamp token: bad genTime %q", raw.GenTime.Bytes)
	}

	return &tstInfo{GenTime: genTime, Nonce: raw.Nonce, MessageImprint: raw.MessageImprint}, nil
}
