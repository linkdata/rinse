package rinser

import (
	"html/template"
	"time"

	"github.com/linkdata/jaws"
)

type uiAutoCleanup struct{ *Rinse }

func (u uiAutoCleanup) Text() string {
	if n := u.AutoCleanup(); n < 1 {
		return "never"
	} else {
		return prettyDuration(time.Minute * time.Duration(n))
	}
}

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiAutoCleanup) JawsGetHtml(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text()) // #nosec G203
}

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

func (rns *Rinse) UiAutoCleanup() jaws.HtmlGetter {
	return uiAutoCleanup{rns}
}
