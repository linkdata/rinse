package jwt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

var ErrInvalidJWTForm = fmt.Errorf("auth token not in JWT format")
var ErrJWTExpired = fmt.Errorf("jwt has expired")
var ErrUnidentifiedUser = fmt.Errorf("no username found with which to identify user")

type JWTHeader struct {
	Kid       string `json:"kid"`
	Algorithm string `json:"alg"`
}

type JWTPayload struct {
	UniqueUsername    string `json:"unique_name,omitempty"`        // Microsoft Token V1.0
	PreferredUsername string `json:"preferred_username,omitempty"` // Microsoft Token V2.0, Keycloak
	Expires           int64  `json:"exp"`                          // UNIX timestamp
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

func GetUsernameFromPayload(jwt string) (string, error) {
	_, p, _, err := extractHeaderPayloadSignature(jwt)
	if err != nil {
		return "", err
	}
	var payload JWTPayload
	err = json.Unmarshal(decodeJWTStringToBytes(p), &payload)
	if err != nil {
		return "", err
	}

	var username string
	if payload.PreferredUsername != "" {
		username = payload.PreferredUsername
	} else if payload.UniqueUsername != "" {
		username = payload.UniqueUsername
	} else {
		err = ErrUnidentifiedUser
	}

	return username, err
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
	if err := json.Unmarshal(decodeJWTStringToBytes(p64), &payload); err != nil {
		return false, err
	}
	expirationDate := time.Unix(payload.Expires, 0)
	now := time.Now().Truncate(time.Second)
	expired := expirationDate.Before(now)
	if expired {
		return false, fmt.Errorf("%w: %s", ErrJWTExpired, expirationDate.String())
	}

	// check header for signing algorithm
	var header JWTHeader
	if err := json.Unmarshal(decodeJWTStringToBytes(h64), &header); err != nil {
		return false, err
	}
	kid := header.Kid
	method := gojwt.GetSigningMethod(header.Algorithm)

	// get public key
	cert, err := FetchX09SignCert(certs, kid)
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
