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

type JSONWebKeySet map[string]JSONWebKey

// Json Web Key set (JWK)
// see https://www.keycloak.org/docs-api/21.1.2/javadocs/constant-values.html
type JSONWebKey struct {
	KeyId        string   `json:"kid"`
	KeyType      string   `json:"kty"`
	Algorithm    string   `json:"alg"`
	PublicKeyUse string   `json:"use"`
	Modulus      string   `json:"n"`
	Exponent     string   `json:"e"`
	X509Cert     []string `json:"x5c"`
}

func (p JSONWebKey) String() string {
	return fmt.Sprintf("\n{\nkid: %s\nalg: %s\nx5c: %s\n}\n", p.KeyId, p.Algorithm, p.X509Cert)
}

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
		return nil, nil, fmt.Errorf("error decoding certificate %q: %w", kid, err)
	}
	if cert, err = x509.ParseCertificate(b); err != nil {
		return nil, nil, fmt.Errorf("error parsing certificate %q: %w", kid, err)
	}
	return cert.PublicKey, &cert.NotAfter, nil
}
