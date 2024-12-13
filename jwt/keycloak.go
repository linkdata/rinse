package jwt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var ErrNoJWTFoundInHeader = fmt.Errorf("no JWT found in header")
var ErrInvalidJWTForm = fmt.Errorf("auth token not in JWT format")

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

func GetKeycloakJWKs(endpoint string) (keys JSONWebKeySet, err error) {
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

	for _, k := range tmp.Keys {
		keys[k.KeyId] = k
	}

	return
}

func (key *JSONWebKey) GetFirstX509Cert()

/*
func (rns *Rinse) FoundJWTInSession(r *http.Request) (string, error) {
	sess := rns.Jaws.GetSession(r)
	sess.Get()
}
*/

/*
func hello() {
	verifyToken := func(token, publicKeyPath string) (bool, error) {
		keyData, err := os.ReadFile(publicKeyPath)
		if err != nil {
			return false, err
		}
		key, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
		if err != nil {
			return false, err
		}

		parts := strings.Split(token, ".")
		err = jwt.SigningMethodRS256.Verify(strings.Join(parts[0:2], "."), decodeJWTStringToBytes(parts[2]), key)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	publicKeyPath := pubkeyPath          //"./keys/rsapub.pem"
	token := GetTokenFromJson(tokenPath) //"./tokens/jwt3.json")

	isValid, err := verifyToken(token, publicKeyPath)
	if err != nil {
		log.Fatal(err)
	}
}
*/
