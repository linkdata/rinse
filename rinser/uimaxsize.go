package rinser

import (
	"html/template"

	"github.com/linkdata/bytecount"
	"github.com/linkdata/jaws"
)

type uiMaxSize struct{ *Rinse }

func (u uiMaxSize) Text() string {
	u.mu.Lock()
	n := int64(u.maxSizeMB) * 1024 * 1024
	u.mu.Unlock()
	if n < 1 {
		return "unlimited"
	} else {
		return bytecount.Sprint(float64(n))
	}
}

// JawsGetHTML implements jaws.HTMLGetter.
func (u uiMaxSize) JawsGetHTML(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text()) // #nosec G203
}

func (u uiMaxSize) JawsGet(e *jaws.Element) (v float64) {
	u.mu.Lock()
	v = float64(u.maxSizeMB)
	u.mu.Unlock()
	return
}

func (u uiMaxSize) JawsSet(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	u.maxSizeMB = int(v)
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiMaxSize() jaws.HTMLGetter {
	return uiMaxSize{rns}
}
