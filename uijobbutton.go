package rinse

import (
	"errors"
	"html/template"

	"github.com/linkdata/jaws"
)

type uiJobButton struct{ *Job }

// JawsClick implements jaws.ClickHandler.
func (ui uiJobButton) JawsClick(e *jaws.Element, name string) (err error) {
	if name == "jobact" {
		switch ui.State() {
		case JobNew:
			return ui.Start()
		case JobFinished, JobFailed:
			return ui.Close()
		}
		return errors.New("not implemented")
	}
	return jaws.ErrEventUnhandled
}

// JawsGetHtml implements jaws.HtmlGetter.
func (ui uiJobButton) JawsGetHtml(rq *jaws.Element) template.HTML {
	switch ui.State() {
	case JobNew:
		return "Start"
	case JobFinished, JobFailed:
		return "Clear"
	}
	return "Stop"
}

func (job *Job) Button() jaws.ClickHandler {
	return uiJobButton{job}
}
