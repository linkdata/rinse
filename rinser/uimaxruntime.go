package rinser

import (
	"html/template"
	"time"

	"github.com/linkdata/jaws"
)

type uiMaxRuntime struct{ *Rinse }

func (u uiMaxRuntime) Text() string {
	if n := u.MaxTimeSec(); n < 1 {
		return "unlimited"
	} else {
		return prettyDuration(time.Second * time.Duration(n))
	}
}

// JawsGetHTML implements jaws.HTMLGetter.
func (u uiMaxRuntime) JawsGetHTML(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text()) // #nosec G203
}

func (u uiMaxRuntime) JawsGet(e *jaws.Element) float64 {
	return float64(u.MaxTimeSec())
}

func (u uiMaxRuntime) JawsSet(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	u.maxTimeSec = int(v)
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiMaxRuntime() jaws.HTMLGetter {
	return uiMaxRuntime{rns}
}
