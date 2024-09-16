package rinse

import (
	"html/template"

	"github.com/linkdata/jaws"
)

type uiMaxSize struct{ *Rinse }

func (u uiMaxSize) Text() string {
	if n := u.MaxUploadSize(); n < 1 {
		return "unlimited"
	} else {
		return prettyByteSize(n)
	}
}

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiMaxSize) JawsGetHtml(rq *jaws.Element) template.HTML {
	return template.HTML(u.Text())
}

// JawsGetFloat implements jaws.FloatSetter.
func (u uiMaxSize) JawsGetFloat(e *jaws.Element) float64 {
	return float64(u.MaxUploadSize() / 1024 / 1024)
}

// JawsSetFloat implements jaws.FloatSetter.
func (u uiMaxSize) JawsSetFloat(e *jaws.Element, v float64) (err error) {
	u.mu.Lock()
	u.maxUploadSize = int64(v) * 1024 * 1024
	u.mu.Unlock()
	return u.saveSettings()
}

func (rns *Rinse) UiMaxSize() jaws.HtmlGetter {
	return uiMaxSize{rns}
}
