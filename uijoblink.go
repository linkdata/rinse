package rinse

import (
	"fmt"
	"html"
	"html/template"
	"path/filepath"

	"github.com/linkdata/jaws"
)

type uiJobLink struct{ *Job }

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiJobLink) JawsGetHtml(rq *jaws.Element) template.HTML {
	var s string
	if u.State() == JobFinished {
		s = fmt.Sprintf(`<a target="_blank" href="/job/%s">%s</a>`, u.UUID, html.EscapeString(u.ResultName()))
	} else {
		s = html.EscapeString(u.Name)
	}
	s += fmt.Sprintf(`<span class="ms-2 badge text-bg-light">%s</span><span class="ms-2 badge text-bg-light">%s</span>`,
		filepath.Ext(u.DocumentName()), u.LanguageName(u.Lang()))
	return template.HTML(s) // #nosec G203
}

func (job *Job) Link() jaws.HtmlGetter {
	return uiJobLink{job}
}
