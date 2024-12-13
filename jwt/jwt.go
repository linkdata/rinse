package jwt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

var ErrInvalidJWTForm = fmt.Errorf("auth token not in JWT format")
var ErrJWTExpired = fmt.Errorf("jwt has expired")

type JWTHeader struct {
	Kid       string `json:"kid"`
	Algorithm string `json:"alg"`
	//Type      string `json:"typ"` //unsure what this should be used for
}

type JWTPayload struct {
	Expires int64 `json:"exp"` // UNIX timestamp
	// Issuer  string `json:"iss"`	//TODO check ie if the issuer is an approved one?
}

// decodeJWTStringToBytes decodes a JWT specific base64url encoding,
// and returns the bytes represented by the base64 string
func decodeJWTStringToBytes(str string) (b []byte) {
	var err error
	b, err = gojwt.NewParser().DecodeSegment(str)
	if err != nil {
		fmt.Printf("could not decode segment: %v", err)
	}
	return
}

func extractHeaderPayloadSignature(jwt string) (header, payload, signature string, err error) {
	jwtSplit := strings.Split(string(jwt), ".")
	if len(jwtSplit) != 3 {
		err = ErrInvalidJWTForm
		return
	}
	header = jwtSplit[0]
	payload = jwtSplit[1]
	signature = jwtSplit[2]
	return
}

// Verify whether a JSON Web Token is valid.
// Takes the token in form of a string and a set of JSON Web Keys (public keys/certs) as input.
func VerifyJWT(jwt string, certs JSONWebKeySet) (bool, error) {
	if len(certs) == 0 {
		return false, ErrNoJWKAvailable
	}

	h64, p64, s64, err := extractHeaderPayloadSignature(jwt)
	if err != nil {
		return false, err
	}

	// check that JWT not expired
	var payload JWTPayload
	json.Unmarshal(decodeJWTStringToBytes(p64), &payload)
	expirationDate := time.Unix(payload.Expires, 0)
	now := time.Now()
	slog.Warn("[DEBUG]", "now", now.String(), "exp", expirationDate.String())
	expired := expirationDate.Before(now)
	if expired {
		return false, fmt.Errorf("%w: %s", ErrJWTExpired, expirationDate.String())
	}

	// check header for signing algorithm
	var header JWTHeader
	json.Unmarshal(decodeJWTStringToBytes(h64), &header)
	kid := header.Kid
	method := gojwt.GetSigningMethod(header.Algorithm)

	// Get public key
	cert, err := FetchX09Cert(certs, kid) //TODO undersök det här med att den är en lista..
	if err != nil {
		return false, err
	}
	pubkey, _, err := ParseX09AsPublicKey(cert, kid)
	if err != nil {
		return false, err
	}

	// verify
	signed := fmt.Sprintf("%s.%s", h64, p64)
	sig := decodeJWTStringToBytes(s64)

	err = method.Verify(signed, sig, pubkey)
	if err != nil {
		return false, err
	}

	return true, nil
}
