package rinse

import (
	"github.com/linkdata/jaws"
)

type uiAutoCleanup struct{ *Rinse }

// JawsGetFloat implements jaws.FloatSetter.
func (u uiAutoCleanup) JawsGetFloat(e *jaws.Element) float64 {
	return float64(u.AutoCleanup())
}

// JawsSetFloat implements jaws.FloatSetter.
func (u uiAutoCleanup) JawsSetFloat(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	u.autoCleanup = int(v)
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiAutoCleanup() jaws.FloatSetter {
	return uiAutoCleanup{rns}
}
