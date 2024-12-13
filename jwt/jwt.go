package jwt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"
)

var ErrNoJWKAvailable = fmt.Errorf("no JWKs (certs or public keys) available")

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

// Parses Authorization header and matches pattern {string}.{string}.{string}
// to find the potential JWT. So if the header looks like e.g. 'Authorization':'Bearer {JWT}'
// only the actual JWT is returned.
// Returns error if not found or invalid format.
func GetJWTFromHeader(r *http.Request) (string, error) {
	var jwtStr string

	header := r.Header
	auth := header.Get("Authorization")
	if auth == "" {
		return "", ErrNoJWTFoundInHeader
	}

	re := regexp.MustCompile(`(^[A-Za-z0-9-_]*\.[A-Za-z0-9-_]*\.[A-Za-z0-9-_]*$)`)
	jwtStr = re.FindString(auth)
	slog.Warn("[DEBUG]", "jwt", jwtStr)
	if jwtStr == "" {
		return "", ErrInvalidJWTForm
	}

	return jwtStr, nil
}

// decodeJWTStringToBytes decodes a JWT specific base64url encoding,
// and returns the bytes represented by the base64 string
func decodeJWTStringToBytes(str string) (b []byte) {
	var err error
	b, err = jwt.NewParser().DecodeSegment(str)
	if err != nil {
		fmt.Printf("could not decode segment: %v", err)
	}
	return
}

// Splits up the JWT string into its components: header, payload and signature.
func ExtractHeaderPayloadSignature(jwtToken string) (header, payload, signature string) {
	tokenSplit := strings.Split(jwtToken, ".")
	header = tokenSplit[0]
	payload = tokenSplit[1]
	signature = tokenSplit[2]
	return
}

// Verify a JWT given a JWT string and a list of JWKs
func VerifyJWT(jwtToken string, certs JSONWebKeySet) (bool, error) {
	if len(certs) == 0 {
		return false, ErrNoJWKAvailable
	}

	h64, p64, s64 := ExtractHeaderPayloadSignature(jwtToken)
	var header JWTHeader
	json.Unmarshal(decodeJWTStringToBytes(h64), &header)

	//TODO check payload, ie is the issuer an approved one

	cert := certs[header.Kid]
	key := cert.X509Cert[0] //TODO undersök det här med att den är en lista..

	signed := fmt.Sprintf("%s.%s", h64, p64)
	sig := decodeJWTStringToBytes(s64)

	/*
		TODO
		get kid from keyMap
		get alg from header and use that to fetch type of method from jwt
	*/

	err := jwt.SigningMethodRS256.Verify(signed, sig, key)
	if err != nil {
		return false, err
	}

}
