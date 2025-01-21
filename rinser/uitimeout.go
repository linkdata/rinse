package rinser

import (
	"html/template"
	"time"

	"github.com/linkdata/jaws"
)

type uiTimeout struct{ *Rinse }

func (u uiTimeout) Text() string {
	if n := u.TimeoutSec(); n < 1 {
		return "unlimited"
	} else {
		return prettyDuration(time.Second * time.Duration(n))
	}
}

// JawsGetHTML implements jaws.HTMLGetter.
func (u uiTimeout) JawsGetHTML(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text()) // #nosec G203
}

func (u uiTimeout) JawsGet(e *jaws.Element) float64 {
	return float64(u.TimeoutSec())
}

func (u uiTimeout) JawsSet(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	u.timeoutSec = int(v)
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiTimeout() jaws.HTMLGetter {
	return uiTimeout{rns}
}
