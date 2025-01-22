package rinser

import (
	"github.com/linkdata/jaws"
)

type uiJobLog struct {
	*Job
}

func (ui uiJobLog) JawsUpdate(e *jaws.Element) {
	if ui.HasLog() {
		e.RemoveAttr("hidden")
	} else {
		e.SetAttr("hidden", "")
	}
}

func (job *Job) UiJobLog() jaws.Updater {
	return uiJobLog{job}
}
