package rinser

import (
	"github.com/linkdata/jaws"
)

type uiJobPreview struct {
	*Job
}

// JawsUpdate implements jaws.Updater.
func (ui uiJobPreview) JawsUpdate(e *jaws.Element) {
	if ui.Previewable() {
		e.RemoveAttr("hidden")
	} else {
		e.SetAttr("hidden", "")
	}
}

func (job *Job) UiJobPreview() jaws.Updater {
	return uiJobPreview{job}
}
