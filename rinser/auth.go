package rinser

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/linkdata/rinse/jwt"
)

var ErrNoJWTFoundInHeader = fmt.Errorf("no JWT found in header")

func (rns *Rinse) AskForAuthFn(fn http.HandlerFunc) http.Handler {
	return rns.JawsAuth.Wrap(http.HandlerFunc(fn))
}

func (rns *Rinse) AuthFn(fn http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rns.CheckAuth(w, r, fn)
	})
}

// Checks for JWT in header, if no JWT is found, redirects to login
// If JWT is found in header but is invalid, error response is return to caller.
func (rns *Rinse) CheckAuth(w http.ResponseWriter, r *http.Request, fn http.HandlerFunc) {
	/*
		If no token is found in header, check whether there is a valid token in session
		If token found but not valid, return error respose
		If no token found in neither header nor session,
	*/
	var (
		token    string
		inHeader bool
		err      error
	)

	token, err = GetJWTFromHeader(r)
	if err == nil {
		inHeader, err = jwt.VerifyJWT(token, rns.JWTPublicKeys)
	}

	if err != nil && !errors.Is(err, ErrNoJWTFoundInHeader) {
		SendHTTPError(w, http.StatusBadRequest, err)
		return
	}

	if inHeader {
		fn(w, r)
		slog.Warn("[DEBUG] fn")
	} else {
		// TODO DEBUG/DEV
		//HTTPJSON(w, http.StatusTeapot, "REDIRECTING")
		fn := rns.JawsAuth.Wrap(http.HandlerFunc(fn))
		fn.ServeHTTP(w, r)
		slog.Warn("[DEBUG] new_fn")
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

	re := regexp.MustCompile(`(^[A-Za-z0-9-_]*\.[A-Za-z0-9-_]*\.[A-Za-z0-9-_]*$)`)
	jwtStr = re.FindString(auth)
	slog.Warn("[DEBUG]", "jwt", jwtStr)
	if jwtStr == "" {
		return "", jwt.ErrInvalidJWTForm
	}

	return jwtStr, nil
}

func (rns *Rinse) FoundValidJWTInSession() (bool, error) {
	token := rns.JawsAuth.SessionTokenKey
	return jwt.VerifyJWT(token, rns.JWTPublicKeys)
}
