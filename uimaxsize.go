package rinse

import (
	"errors"

	"github.com/linkdata/jaws"
)

type uiMaxSize struct{ *Rinse }

// JawsGetFloat implements jaws.FloatSetter.
func (u uiMaxSize) JawsGetFloat(e *jaws.Element) float64 {
	return float64(u.MaxUploadSize() / 1024 / 1024)
}

// JawsSetFloat implements jaws.FloatSetter.
func (u uiMaxSize) JawsSetFloat(e *jaws.Element, v float64) (err error) {
	if x := int64(v); x > 0 {
		u.mu.Lock()
		u.maxUploadSize = int64(x) * 1024 * 1024
		u.mu.Unlock()
		return u.saveSettings()
	}
	return errors.New("minimum upload size is 1MB")
}

func (rns *Rinse) UiMaxSize() jaws.FloatSetter {
	return uiMaxSize{rns}
}
