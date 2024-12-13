package jwt

import (
	"encoding/json"
	"fmt"
	"strings"

	gojwt "github.com/golang-jwt/jwt/v5"
)

type JWTHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	Kid       string `json:"kid"`
}

type JWTPayload struct {
	Issuer string `json:"iss"`
	Type   string `json:"typ"`
	// TODO add on more here
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

// Verify a JWT given a JWT string and a list of JWKs
func VerifyJWT(jwt string, certs JSONWebKeySet) (bool, error) {
	if len(certs) == 0 {
		return false, ErrNoJWKAvailable
	}

	tokenSplit := strings.Split(string(jwt), ".")
	h64 := tokenSplit[0]
	p64 := tokenSplit[1]
	s64 := tokenSplit[2]

	var header JWTHeader
	json.Unmarshal(decodeJWTStringToBytes(h64), &header)

	//TODO check payload, ie is the issuer an approved one
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
