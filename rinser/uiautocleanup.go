package rinser

import (
	"html/template"
	"time"

	"github.com/linkdata/jaws"
)

type uiAutoCleanup struct{ *Rinse }

func (u uiAutoCleanup) Text() string {
	n := u.CleanupSec()

	if n < 0 {
		return "never"
	} else if n == 0 {
		return "immediately"
	}
	return prettyDuration(time.Second * time.Duration(n))
}

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiAutoCleanup) JawsGetHtml(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text()) // #nosec G203
}

func (u uiAutoCleanup) JawsGet(e *jaws.Element) float64 {
	return float64(u.CleanupSec())
}

func (u uiAutoCleanup) JawsSet(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	u.cleanupSec = int(v)
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiAutoCleanup() jaws.HtmlGetter {
	return uiAutoCleanup{rns}
}
