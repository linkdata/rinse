package rinse

import (
	"fmt"
	"html"
	"html/template"

	"github.com/linkdata/jaws"
)

type uiJobLink struct{ *Job }

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiJobLink) JawsGetHtml(rq *jaws.Element) template.HTML {
	if u.State() == JobFinished {
		return template.HTML(fmt.Sprintf(`<a href="/get/%s">%s</a>`, u.UUID, html.EscapeString(u.ResultName)))
	}
	return template.HTML(html.EscapeString(u.Name))
}

func (job *Job) Link() jaws.HtmlGetter {
	return uiJobLink{job}
}
