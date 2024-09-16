package rinse

import (
	"html/template"
	"strconv"

	"github.com/linkdata/jaws"
)

type uiMaxConcurrent struct{ *Rinse }

func (u uiMaxConcurrent) Text() string {
	return strconv.Itoa(u.MaxConcurrent())
}

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiMaxConcurrent) JawsGetHtml(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text())
}

// JawsGetFloat implements jaws.FloatSetter.
func (u uiMaxConcurrent) JawsGetFloat(e *jaws.Element) float64 {
	return float64(u.MaxConcurrent())
}

// JawsSetFloat implements jaws.FloatSetter.
func (u uiMaxConcurrent) JawsSetFloat(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	if n := int(v); n > 0 {
		u.maxConcurrent = n
	}
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiMaxConcurrent() jaws.HtmlGetter {
	return uiMaxConcurrent{rns}
}
