package rinse

import (
	"html/template"

	"github.com/linkdata/jaws"
)

type uiPreview struct {
	*Job
}

// JawsUpdate implements jaws.Updater.
func (ui uiPreview) JawsUpdate(e *jaws.Element) {
	if ui.Previewable() {
		e.RemoveAttr("hidden")
	} else {
		e.SetAttr("hidden", "")
	}
}

func (job *Job) UiPreview() jaws.Updater {
	return uiPreview{job}
}

func (job *Job) UiPreviewAttr() template.HTMLAttr {
	if !job.Previewable() {
		return "hidden"
	}
	return ""
}
