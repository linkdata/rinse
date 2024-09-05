package rinse

import (
	"fmt"
	"html/template"

	"github.com/linkdata/jaws"
)

type uiJobPagecount struct{ *Job }

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiJobPagecount) JawsGetHtml(e *jaws.Element) template.HTML {
	u.mu.Lock()
	diskuse := u.diskuse
	state := u.state
	todo := len(u.ppmtodo)
	done := len(u.ppmdone)
	u.mu.Unlock()
	if todo == 0 && state == JobPdfToPPm {
		diskuse, todo = u.getDiskuse()
		u.mu.Lock()
		u.diskuse = diskuse
		u.mu.Unlock()
	}
	diskuseflt := float64(diskuse)
	diskusesuffix := "B"
	switch {
	case diskuse > 1024*1024*1024:
		diskuseflt /= (1024 * 1024 * 1024)
		diskusesuffix = "GB"
	case diskuse > 1024*1024:
		diskuseflt /= (1024 * 1024)
		diskusesuffix = "MB"
	case diskuse > 1024:
		diskuseflt /= (1024)
		diskusesuffix = "KB"
	}
	return template.HTML(fmt.Sprintf(`<span data-toggle="tooltip" title="%.2f%s">%d/%d</span>`, diskuseflt, diskusesuffix, done, todo+done))
}

func (job *Job) Pagecount() (ui jaws.HtmlGetter) {
	return uiJobPagecount{job}
}
