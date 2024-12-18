package rinser

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/linkdata/rinse/jwt"
)

var ErrNoJWTFoundInHeader = fmt.Errorf("no JWT found in header")

func (rns *Rinse) RedirectAuthFn(fn http.HandlerFunc) http.Handler {
	return rns.JawsAuth.Wrap(http.HandlerFunc(fn))
}

func (rns *Rinse) AuthFn(fn http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rns.CheckAuth(w, r, fn)
	})
}

func (rns *Rinse) setUsernameInSession(w http.ResponseWriter, r *http.Request, username string) {
	sess := rns.Jaws.GetSession(r)
	if sess == nil {
		sess = rns.Jaws.NewSession(w, r)
	}
	sess.Set(rns.JawsAuth.SessionEmailKey, username)
}

// Checks for JWT in header, if no JWT is found, redirects to login
// If JWT is found in header but is invalid, error response is return to caller.
// If JWT is found in header and valid, sets EmailKey in session to the 'username' gotten from the JWT
func (rns *Rinse) CheckAuth(w http.ResponseWriter, r *http.Request, fn http.HandlerFunc) {
	var (
		token    string
		inHeader bool
		err      error
	)

	token, err = GetJWTFromHeader(r)
	if err == nil {
		inHeader, err = jwt.VerifyJWT(token, rns.JWTPublicKeys)
		if err == nil {
			// sets username in session in order to get fine-grain control
			// over what a user can access
			var username string
			username, err = jwt.GetUsernameFromPayload(token)
			if err == nil {
				rns.setUsernameInSession(w, r, username)
			}
		}
	}

	if err != nil && !errors.Is(err, ErrNoJWTFoundInHeader) {
		SendHTTPError(w, http.StatusBadRequest, err)
		return
	}

	if inHeader {
		fn(w, r)
	} else {
		fn := rns.RedirectAuthFn(fn)
		fn.ServeHTTP(w, r)
	}
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

	// The regexp matches the JWT pattern 'header.payload.signature'
	// each of which element is a base64URL
	// RFC4648 https://datatracker.ietf.org/doc/html/rfc4648#section-5
	// \w is equivalent [A-Za-z0-9-_]
	re, err := regexp.Compile(`([\w\-\_]+\.[\w\-\_]+\.[\w\-\_]+)`)
	if err != nil {
		return "", jwt.ErrInvalidJWTForm
	}

	jwtStr = re.FindString(auth)
	if jwtStr == "" {
		return "", jwt.ErrInvalidJWTForm
	}

	return jwtStr, nil
}

func (rns *Rinse) FoundValidJWTInSession() (bool, error) {
	token := rns.JawsAuth.SessionTokenKey
	return jwt.VerifyJWT(token, rns.JWTPublicKeys)
}
