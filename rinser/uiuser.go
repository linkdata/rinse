package rinser

import (
	"fmt"
	"html"
	"html/template"

	"github.com/linkdata/jaws"
)

type uiUser struct{ *Rinse }

func (ui uiUser) JawsGetHtml(e *jaws.Element) template.HTML {
	textClass := "text-secondary"
	email := ui.GetEmail(e.Initial())
	if ui.IsAdmin(email) {
		textClass += " fw-bold"
	}
	return template.HTML(fmt.Sprintf(`<span class="%s">%s</span>`, textClass, html.EscapeString(email))) //#nosec G203
}

func (rns *Rinse) UiUser() jaws.HtmlGetter {
	return uiUser{rns}
}
