package jwt

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var (
	ErrNoJWKAvailable      = fmt.Errorf("no JWKs (certs or public keys) available")
	ErrNoMatchingJWKFound  = fmt.Errorf("no JWK with mathing KeyId found")
	ErrUnknownKeyType      = fmt.Errorf("JWK key of unknown type")
	ErrFailedToParseCertFn = func(kid string, err error) error { return fmt.Errorf("error decoding certificate %q: %w", kid, err) }
)

type JSONWebKey struct {
	KeyId        string   `json:"kid"`
	KeyType      string   `json:"kty"`
	Algorithm    string   `json:"alg"`
	PublicKeyUse string   `json:"use"`
	X509Cert     []string `json:"x5c"`
}

type JSONWebKeySet map[string]JSONWebKey

func GetJSONKeyWebSet(endpoint string) (jwks JSONWebKeySet, err error) {
	var resp *http.Response
	resp, err = http.Get(endpoint)
	if err != nil {
		return
	}

	var body []byte
	body, err = io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return
	}

	var tmp struct {
		Keys []JSONWebKey `json:"keys"`
	}
	err = json.Unmarshal(body, &tmp)
	if err != nil {
		return
	}

	jwks = make(map[string]JSONWebKey, 0)
	for _, k := range tmp.Keys {
		jwks[k.KeyId] = k
	}

	return
}

func FetchX09Cert(keys JSONWebKeySet, kid string) (string, error) {
	cert, ok := keys[kid]
	if !ok {
		return "", ErrNoMatchingJWKFound
	}

	certs := cert.X509Cert
	if len(certs) == 0 {
		return "", ErrNoJWKAvailable
	}
	return certs[0], nil
}

func ParseX09AsPublicKey(key, kid string) (any, *time.Time, error) {
	var (
		b    []byte
		cert *x509.Certificate
		err  error
	)

	if b, err = base64.StdEncoding.DecodeString(key); err != nil {
		return nil, nil, ErrFailedToParseCertFn(kid, err)
	}
	if cert, err = x509.ParseCertificate(b); err != nil {
		return nil, nil, ErrFailedToParseCertFn(kid, err)
	}
	return cert.PublicKey, &cert.NotAfter, nil
}
