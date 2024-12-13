package rinser

import (
	"log/slog"
	"net/http"
	"regexp"

	"github.com/linkdata/rinse/jwt"
)

// Parses Authorization header and matches pattern {string}.{string}.{string}
// to find the potential JWT. So if the header looks like e.g. 'Authorization':'Bearer {JWT}'
// only the actual JWT is returned.
// Returns error if not found or invalid format.
func GetJWTFromHeader(r *http.Request) (string, error) {
	var jwtStr string

	header := r.Header
	auth := header.Get("Authorization")
	if auth == "" {
		return "", jwt.ErrNoJWTFoundInHeader
	}

	re := regexp.MustCompile(`(^[A-Za-z0-9-_]*\.[A-Za-z0-9-_]*\.[A-Za-z0-9-_]*$)`)
	jwtStr = re.FindString(auth)
	slog.Warn("[DEBUG]", "jwt", jwtStr)
	if jwtStr == "" {
		return "", jwt.ErrInvalidJWTForm
	}

	return jwtStr, nil
}

/*
func (rns *Rinse) FoundJWTInSession(r *http.Request) (string, error) {
	sess := rns.Jaws.GetSession(r)
	sess.Get()
}
*/
