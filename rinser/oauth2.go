package rinser

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"strings"

	"golang.org/x/oauth2"
)

type OAuth2Settings struct {
	RedirectHost string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	Scope        string
	UserInfoURL  string
}

type oAuth2User struct {
	DisplayName string `json:"displayName,omitempty"`
	Mail        string `json:"mail,omitempty"`
}

func (settings *OAuth2Settings) Valid() bool {
	return settings != nil &&
		settings.RedirectHost != "" &&
		settings.ClientID != "" &&
		settings.ClientSecret != "" &&
		settings.AuthURL != "" &&
		settings.TokenURL != "" &&
		settings.Scope != "" &&
		settings.UserInfoURL != ""
}

func (settings *OAuth2Settings) Config(hostPort string) (cfg *oauth2.Config) {
	if settings.Valid() {
		_, portstr, _ := net.SplitHostPort(hostPort)
		scheme := "https"
		host := settings.RedirectHost
		switch portstr {
		case "443", "8443":
		case "80", "8080":
			scheme = "http"
		default:
			host = "localhost:" + portstr
			if !strings.Contains(portstr, "443") {
				scheme = "http"
			}
		}
		cfg = &oauth2.Config{
			ClientID:     settings.ClientID,
			ClientSecret: settings.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  settings.AuthURL,
				TokenURL: settings.TokenURL,
			},
			RedirectURL: scheme + "://" + host + "/auth-response",
			Scopes:      []string{settings.Scope},
		}
	}
	return
}

type OAuth2Handler struct {
	*Rinse
	http.Handler
}

func (rns *Rinse) Authed(h http.Handler) http.Handler {
	return OAuth2Handler{Rinse: rns, Handler: h}
}

func (rns *Rinse) AuthFn(h http.HandlerFunc) http.Handler {
	return OAuth2Handler{Rinse: rns, Handler: h}
}

func (oh OAuth2Handler) ServeHTTP(hw http.ResponseWriter, hr *http.Request) {
	oh.mu.Lock()
	oauth2Config := oh.oauth2Config
	oh.mu.Unlock()

	sess := oh.Jaws.GetSession(hr)
	if sess == nil {
		sess = oh.Jaws.NewSession(hw, hr)
	}
	if _, ok := sess.Get("user").(string); !ok {
		if oauth2Config != nil {
			oh.HandleLogin(hw, hr)
			return
		}
	}
	oh.Handler.ServeHTTP(hw, hr)
}

var ErrOAuth2NotConfigured = errors.New("OAuth2 not configured")
var ErrSessionNotFound = errors.New("session not found")

const oauth2ReferrerKey = "oauth2referrer"
const oauth2StateKey = "oauth2state"

var oauth2Paths = []string{
	"/login",
	"/logout",
	"/auth-response",
}

func oauth2referer(hr *http.Request) string {
	location := strings.TrimSpace(hr.Referer())
	for _, p := range oauth2Paths {
		location = strings.TrimSuffix(location, p)
	}
	if location == "" {
		location = "/"
	}
	return location
}

func (rns *Rinse) HandleLogin(hw http.ResponseWriter, hr *http.Request) {
	rns.mu.Lock()
	oauth2Config := rns.oauth2Config
	rns.mu.Unlock()

	url := oauth2referer(hr)
	if oauth2Config != nil {
		if sess := rns.Jaws.GetSession(hr); sess != nil {
			b := make([]byte, 4)
			n, _ := rand.Read(b)
			state := fmt.Sprintf("%x%#p", b[:n], rns)
			sess.Set(oauth2StateKey, state)
			sess.Set(oauth2ReferrerKey, url)
			url = oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
		}
	}
	hw.Header().Add("Location", url)
	hw.WriteHeader(http.StatusFound)
	return
}

func (rns *Rinse) HandleLogout(hw http.ResponseWriter, hr *http.Request) {
	if sess := rns.Jaws.GetSession(hr); sess != nil {
		if cookie := sess.Close(); cookie != nil {
			http.SetCookie(hw, cookie)
		}
		sess.Clear()
		rns.Jaws.Dirty(rns.UiUser())
	}
	hw.Header().Add("Location", oauth2referer(hr))
	hw.WriteHeader(http.StatusFound)
}

func requireCorrectState(gotState, wantState string) error {
	if wantState == "" || wantState != gotState {
		return fmt.Errorf("oauth2: got session state %q, wanted %q", gotState, wantState)
	}
	return nil
}

func (rns *Rinse) HandleAuthResponse(hw http.ResponseWriter, hr *http.Request) {
	rns.mu.Lock()
	oauth2Config := rns.oauth2Config
	oauth2Settings := rns.OAuth2Settings
	rns.mu.Unlock()

	location := oauth2referer(hr)

	if oauth2Config != nil {
		if sess := rns.Jaws.GetSession(hr); sess != nil {
			var err error
			closeSession := true
			defer func() {
				if closeSession {
					if cookie := sess.Close(); cookie != nil {
						http.SetCookie(hw, cookie)
					}
					sess.Clear()
				}
			}()

			gotState := hr.FormValue("state")
			wantState, _ := sess.Get(oauth2StateKey).(string)
			sess.Set(oauth2StateKey, nil)

			if err = requireCorrectState(gotState, wantState); err == nil {
				var token *oauth2.Token
				if token, err = rns.oauth2Config.Exchange(context.Background(), hr.FormValue("code")); err == nil {
					client := rns.oauth2Config.Client(context.Background(), token)
					var resp *http.Response
					if resp, err = client.Get(oauth2Settings.UserInfoURL); err == nil {
						var b []byte
						if b, err = io.ReadAll(resp.Body); err == nil {
							var u oAuth2User
							if err = json.Unmarshal(b, &u); err == nil {
								var mailaddr *mail.Address
								if mailaddr, err = mail.ParseAddress(u.Mail); err == nil {
									closeSession = false
									sess.Set("user", mailaddr.String())
									if s, ok := sess.Get(oauth2ReferrerKey).(string); ok {
										location = s
									}
									sess.Set(oauth2ReferrerKey, nil)
									slog.Info("login", "user", mailaddr, "sess", sess.ID())
									rns.Jaws.Dirty(rns.UiUser())
								}
							}
						}
					}
				}
			}
			if err != nil {
				if cookie := sess.Close(); cookie != nil {
					http.SetCookie(hw, cookie)
				}
				sess.Clear()
				slog.Error("HandleAuthResponse", "err", err)
				hw.WriteHeader(http.StatusBadRequest)
				rns.Jaws.Dirty(rns.UiUser())
				return
			}
		}
	}

	hw.Header().Add("Location", location)
	hw.WriteHeader(http.StatusFound)
}
