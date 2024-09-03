package rinse

import (
	"fmt"
	"html/template"

	"github.com/linkdata/jaws"
)

type uiJobPagecount struct{ *Job }

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiJobPagecount) JawsGetHtml(e *jaws.Element) template.HTML {
	u.mu.Lock()
	todo := len(u.ppmtodo)
	done := len(u.ppmdone)
	u.mu.Unlock()
	return template.HTML(fmt.Sprintf("%d/%d", done, todo+done))
}

func (job *Job) Pagecount() (ui jaws.HtmlGetter) {
	return uiJobPagecount{job}
}
