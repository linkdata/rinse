package rinser

import (
	"html/template"

	"github.com/linkdata/jaws"
)

type uiJobButton struct{ *Job }

// JawsClick implements jaws.ClickHandler.
func (ui uiJobButton) JawsClick(e *jaws.Element, name string) (err error) {
	if name == "jobact" {
		if ui.State() == JobNew {
			return ui.Start()
		}
		ui.Rinse.RemoveJob(ui.Job)
		return nil
	}
	return jaws.ErrEventUnhandled
}

// JawsGetHTML implements jaws.HTMLGetter.
func (ui uiJobButton) JawsGetHTML(rq *jaws.Element) template.HTML {
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
