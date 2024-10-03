package rinser

import (
	"github.com/linkdata/jaws"
)

type uiJobMeta struct {
	*Job
}

// JawsUpdate implements jaws.Updater.
func (ui uiJobMeta) JawsUpdate(e *jaws.Element) {
	if ui.HasMeta() {
		e.RemoveAttr("hidden")
	} else {
		e.SetAttr("hidden", "")
	}
}

func (job *Job) UiJobMeta() jaws.Updater {
	return uiJobMeta{job}
}
