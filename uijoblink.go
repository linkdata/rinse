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
		return template.HTML(fmt.Sprintf(`<a href="/get/%s">%s</a><sup class="text-secondary">&nbsp;%s</sup>`, u.UUID, html.EscapeString(u.ResultName), u.Lang))
	}
	return template.HTML(fmt.Sprintf(`%s<sup class="text-secondary">&nbsp;%s</sup>`, html.EscapeString(u.Name), u.Lang))
}

func (job *Job) Link() jaws.HtmlGetter {
	return uiJobLink{job}
}
