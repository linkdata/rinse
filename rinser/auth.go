package rinser

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
)

var ErrNoJWTFoundInHeader = fmt.Errorf("no JWT found in header")
var ErrInvalidJWTForm = fmt.Errorf("auth token not in JWT format")

type KeycloakPublicKey struct {
	Kid string   `json:"kid"`
	Kty string   `json:"kty"`
	Alg string   `json:"alg"`
	Use string   `json:"use"`
	X5c []string `json:"x5c"`
}

func (p KeycloakPublicKey) String() string {
	return fmt.Sprintf("\n{\nkid: %s\nkty: %s\nalg: %s\nx5c: %s\n}\n", p.Kid, p.Kty, p.Alg, p.X5c)
}

type KeycloakPubKeys map[string]KeycloakPublicKey

func GetKeycloakSigningPubKeys(endpoint string) (keys KeycloakPubKeys, err error) {
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
		Keys []KeycloakPublicKey `json:"keys"`
	}
	err = json.Unmarshal(body, &tmp)

	for _, k := range tmp.Keys {
		keys[k.Kid] = k
	}

	return
}

func GetJWTFromHeader(r *http.Request) (string, error) {
	header := r.Header
	auth := header.Get("Authorization")
	slog.Warn("[DEBUG]", "header", auth)
	if auth == "" {
		return "", ErrNoJWTFoundInHeader
	}

	re := regexp.MustCompile(`(^[A-Za-z0-9-_]*\.[A-Za-z0-9-_]*\.[A-Za-z0-9-_]*$)`)
	jwt := re.FindString(auth)
	slog.Warn("[DEBUG]", "jwt", jwt)
	if jwt == "" {
		return "", ErrInvalidJWTForm
	}

	return jwt, nil
}

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
