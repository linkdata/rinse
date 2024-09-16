package rinse

import (
	"html/template"
	"time"

	"github.com/linkdata/jaws"
)

type uiMaxRuntime struct{ *Rinse }

func (u uiMaxRuntime) Text() string {
	if n := u.MaxRuntime(); n < 1 {
		return "unlimited"
	} else {
		return prettyDuration(time.Second * time.Duration(n))
	}
}

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiMaxRuntime) JawsGetHtml(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text()) // #nosec G203
}

// JawsGetFloat implements jaws.FloatSetter.
func (u uiMaxRuntime) JawsGetFloat(e *jaws.Element) float64 {
	return float64(u.MaxRuntime())
}

// JawsSetFloat implements jaws.FloatSetter.
func (u uiMaxRuntime) JawsSetFloat(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	u.maxRuntime = int(v)
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiMaxRuntime() jaws.HtmlGetter {
	return uiMaxRuntime{rns}
}
