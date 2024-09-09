package rinse

import (
	"fmt"
	"html/template"

	"github.com/linkdata/jaws"
)

type uiJobStatus struct{ *Job }

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiJobStatus) JawsGetHtml(e *jaws.Element) template.HTML {
	u.mu.Lock()
	diskuse := u.diskuse
	state := u.state
	todo := len(u.ppmtodo)
	done := len(u.ppmdone)
	u.mu.Unlock()

	var statetxt string
	switch state {
	case JobNew:
		statetxt = "Waiting"
	case JobStarting:
		statetxt = "Starting"
	case JobDocToPdf:
		statetxt = "Converting"
	case JobPdfToPPm:
		statetxt = fmt.Sprintf("Rendering %d", todo+done)
	case JobTesseract:
		statetxt = fmt.Sprintf("Scanning %d/%d", done, todo+done)
	case JobFailed:
		statetxt = "Failed"
	case JobFinished:
		statetxt = "Done"
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

	return template.HTML(fmt.Sprintf(`%s (%.2f%s)`, statetxt, diskuseflt, diskusesuffix))
}

func (job *Job) Status() (ui jaws.HtmlGetter) {
	return uiJobStatus{job}
}
