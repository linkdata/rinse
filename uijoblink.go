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
	var s string
	if u.State() == JobFinished {
		s = fmt.Sprintf(`<a href="/get/%s">%s</a>`, u.UUID, html.EscapeString(u.ResultName))
	} else {
		s = html.EscapeString(u.Name)
	}
	s += fmt.Sprintf(`<span class="ms-2 badge text-bg-light">%s</span>`, u.LanguageName(u.Lang))
	return template.HTML(s)
}

func (job *Job) Link() jaws.HtmlGetter {
	return uiJobLink{job}
}