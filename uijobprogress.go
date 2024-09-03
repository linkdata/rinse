package rinse

import (
	"fmt"
	"html/template"

	"github.com/linkdata/jaws"
)

type uiJobProgress struct{ *Job }

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiJobProgress) JawsGetHtml(e *jaws.Element) template.HTML {
	e.SetAttr("style", fmt.Sprintf("width: %d%%", u.progress(e)))
	return ""
}

func (job *Job) Progress() (ui jaws.HtmlGetter) {
	return uiJobProgress{job}
}
