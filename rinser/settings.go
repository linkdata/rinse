package rinser

import (
	"encoding/json"
	"errors"
	"os"
	"path"

	"github.com/linkdata/jawsauth"
)

type settings struct {
	MaxSizeMB     int
	CleanupSec    int
	MaxTimeSec    int
	MaxConcurrent int
	CleanupGotten bool
	OAuth2        jawsauth.Config
	ProxyURL      string
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
		MaxConcurrent: rns.maxConcurrent,
		CleanupGotten: rns.cleanupGotten,
		OAuth2:        rns.OAuth2Settings,
		ProxyURL:      rns.proxyUrl,
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
		MaxTimeSec:    3600,
		MaxConcurrent: 2,
		CleanupGotten: true,
	}
	var b []byte
	if b, err = os.ReadFile(rns.SettingsFile()); err == nil {
		err = json.Unmarshal(b, &x)
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	rns.mu.Lock()
	defer rns.mu.Unlock()
	rns.maxSizeMB = min(2048, max(0, x.MaxSizeMB))
	rns.cleanupSec = max(0, x.CleanupSec)
	rns.maxTimeSec = max(0, x.MaxTimeSec)
	rns.maxConcurrent = max(1, x.MaxConcurrent)
	rns.cleanupGotten = x.CleanupGotten
	rns.OAuth2Settings = x.OAuth2
	rns.proxyUrl = x.ProxyURL
	return
}
