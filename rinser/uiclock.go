package rinser

import (
	"html/template"
	"sync"
	"time"

	"github.com/linkdata/jaws"
)

type uiClock struct{}

var uiClockStart sync.Once

func (ui uiClock) JawsGetHtml(e *jaws.Element) template.HTML {
	uiClockStart.Do(func() {
		go func(jw *jaws.Jaws) {
			for {
				now := time.Now()
				time.Sleep(time.Second - now.Sub(now.Truncate(time.Second)))
				jw.Dirty(ui)
			}
		}(e.Jaws)
	})
	now := time.Now().Round(time.Second)
	tformat := "15:04&nbsp;MST"
	if (now.Second() % 2) == 0 {
		tformat = "15&nbsp;04 MST"
	}
	return template.HTML(now.Format(tformat)) // #nosec G203
}

func (rns *Rinse) UiClock() jaws.HtmlGetter {
	return uiClock{}
}
