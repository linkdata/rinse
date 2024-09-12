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
}

func (rns *Rinse) saveSettings() (err error) {
	rns.mu.Lock()
	x := settings{
		MaxUploadSize: rns.maxUploadSize,
		AutoCleanup:   rns.autoCleanup,
	}
	rns.mu.Unlock()
	var b []byte
	if b, err = json.MarshalIndent(x, "", " "); err == nil {
		err = os.WriteFile(path.Join(rns.Config.DataDir, "settings.json"), b, 0664)
	}
	return
}

func (rns *Rinse) loadSettings() (err error) {
	var b []byte
	if b, err = os.ReadFile(path.Join(rns.Config.DataDir, "settings.json")); err == nil {
		var x settings
		if err = json.Unmarshal(b, &x); err == nil {
			rns.mu.Lock()
			defer rns.mu.Unlock()
			rns.maxUploadSize = max(1024*1024, x.MaxUploadSize)
			rns.autoCleanup = max(0, x.AutoCleanup)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return
}
