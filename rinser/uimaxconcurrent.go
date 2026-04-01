package rinser

import (
	"html/template"
	"strconv"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type uiMaxConcurrent struct{ *Rinse }

func (u uiMaxConcurrent) Text() string {
	return strconv.Itoa(u.MaxConcurrent())
}

// JawsGetHTML implements bind.HTMLGetter.
func (u uiMaxConcurrent) JawsGetHTML(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text()) // #nosec G203
}

func (u uiMaxConcurrent) JawsGet(e *jaws.Element) float64 {
	return float64(u.MaxConcurrent())
}

func (u uiMaxConcurrent) JawsSet(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	if n := int(v); n > 0 {
		u.maxConcurrent = n
	}
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiMaxConcurrent() bind.HTMLGetter {
	return uiMaxConcurrent{rns}
}
