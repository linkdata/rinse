package rinse

import (
	"html/template"
	"time"

	"github.com/linkdata/jaws"
)

type uiJobButton struct{ *Job }

// JawsClick implements jaws.ClickHandler.
func (ui uiJobButton) JawsClick(e *jaws.Element, name string) (err error) {
	if name == "jobact" {
		if ui.State() == JobNew {
			return ui.Start(time.Duration(ui.MaxRuntime()) * time.Second)
		}
		ui.RemoveJob(ui.Job)
		return nil
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
