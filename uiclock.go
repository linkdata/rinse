package rinse

import (
	"html/template"
	"time"

	"github.com/linkdata/jaws"
)

type uiClock struct{}

func (ui uiClock) JawsGetHtml(e *jaws.Element) (val template.HTML) {
	now := time.Now().Round(time.Second)
	fmt := "15:04"
	if (now.Second() % 2) == 0 {
		fmt = "15&nbsp;04"
	}
	return template.HTML(now.Format(fmt))
}

func (rns *Rinse) UiClock() jaws.HtmlGetter {
	return uiClock{}
}
