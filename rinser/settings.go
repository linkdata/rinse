package rinser

import (
	"encoding/json"
	"errors"
	"os"
	"path"

	"github.com/linkdata/jawsauth"
)

type settings struct {
	MaxSizeMB       int
	CleanupSec      int
	MaxTimeSec      int
	TimeoutSec      int
	MaxConcurrent   int
	CleanupGotten   bool
	OAuth2          jawsauth.Config
	ProxyURL        string
	Admins          []string
	EndpointForJWKs string // endpoint for getting JWKs used for JWT verification e.g. {keycloak-root-endpoint}/realms/{realm-name}/protocol/openid-connect/certs
}

func (rns *Rinse) SettingsFile() string {
	return path.Join(rns.Config.DataDir, "rinse.json")
}

func (rns *Rinse) saveSettings() (err error) {
	rns.mu.Lock()
	x := settings{
		MaxSizeMB:     rns.maxSizeMB,
		CleanupSec:    rns.cleanupSec,
		MaxTimeSec:    rns.maxTimeSec,
		TimeoutSec:    rns.timeoutSec,
		MaxConcurrent: rns.maxConcurrent,
		CleanupGotten: rns.cleanupGotten,
		OAuth2:        rns.OAuth2Settings,
		ProxyURL:      rns.proxyUrl,
		Admins:        rns.getAdmins(),
	}
	rns.mu.Unlock()
	var b []byte
	if b, err = json.MarshalIndent(x, "", " "); err == nil {
		err = os.WriteFile(rns.SettingsFile(), b, 0664) // #nosec G306
	}
	return
}

func (rns *Rinse) loadSettings() (err error) {
	x := settings{
		MaxSizeMB:     2048,
		CleanupSec:    86400,
		MaxTimeSec:    86400,
		TimeoutSec:    60,
		MaxConcurrent: 2,
		CleanupGotten: true,
	}
	var b []byte
	if b, err = os.ReadFile(rns.SettingsFile()); err == nil {
		err = json.Unmarshal(b, &x)
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
		rns.Config.Logger.Info("No settings file found.")
	}
	rns.mu.Lock()
	defer rns.mu.Unlock()
	rns.maxSizeMB = min(2048, max(0, x.MaxSizeMB))
	rns.cleanupSec = max(0, x.CleanupSec)
	rns.maxTimeSec = max(0, x.MaxTimeSec)
	rns.timeoutSec = max(0, x.TimeoutSec)
	rns.maxConcurrent = max(1, x.MaxConcurrent)
	rns.cleanupGotten = x.CleanupGotten
	rns.OAuth2Settings = x.OAuth2
	rns.proxyUrl = x.ProxyURL
	rns.admins = x.Admins
	rns.endpointForJWKs = x.EndpointForJWKs
	return
}
