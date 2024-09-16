package rinse

import (
	"encoding/json"
	"errors"
	"os"
	"path"
)

type settings struct {
	MaxUploadSize int64
	AutoCleanup   int
	MaxRuntime    int
}

func (rns *Rinse) settingsFile() string {
	return path.Join(rns.Config.DataDir, "settings.json")
}

func (rns *Rinse) saveSettings() (err error) {
	rns.mu.Lock()
	x := settings{
		MaxUploadSize: rns.maxUploadSize,
		AutoCleanup:   rns.autoCleanup,
		MaxRuntime:    rns.maxRuntime,
	}
	rns.mu.Unlock()
	var b []byte
	if b, err = json.MarshalIndent(x, "", " "); err == nil {
		err = os.WriteFile(rns.settingsFile(), b, 0664)
	}
	return
}

func (rns *Rinse) loadSettings() (err error) {
	var b []byte
	if b, err = os.ReadFile(rns.settingsFile()); err == nil {
		var x settings
		if err = json.Unmarshal(b, &x); err == nil {
			rns.mu.Lock()
			defer rns.mu.Unlock()
			rns.maxUploadSize = max(1024*1024, x.MaxUploadSize)
			rns.autoCleanup = max(0, x.AutoCleanup)
			rns.maxRuntime = max(0, x.MaxRuntime)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return
}
