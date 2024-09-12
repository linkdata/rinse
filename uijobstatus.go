package rinse

import (
	"fmt"
	"html"
	"html/template"

	"github.com/linkdata/jaws"
)

type uiJobStatus struct{ *Job }

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiJobStatus) JawsGetHtml(e *jaws.Element) template.HTML {
	u.mu.Lock()
	diskuse := u.diskuse
	state := u.state
	ppmcount := len(u.ppmfiles)
	var ppmdone int
	for _, seen := range u.ppmfiles {
		if seen {
			ppmdone++
		}
	}
	u.mu.Unlock()

	var statetxt string
	switch state {
	case JobNew:
		statetxt = "Waiting"
	case JobStarting:
		statetxt = "Starting"
	case JobDetect:
		statetxt = "Detect Language"
	case JobDocToPdf:
		statetxt = "Converting"
	case JobPdfToPPm:
		statetxt = fmt.Sprintf("Rendered %d", ppmcount)
	case JobTesseract:
		statetxt = fmt.Sprintf("Scanning %d/%d", ppmdone, ppmcount)
	case JobEnding:
		statetxt = "Cleanup"
	case JobFinished:
		statetxt = "Finished"
	case JobFailed:
		statetxt = "Failed"
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

	s := html.EscapeString(fmt.Sprintf(`%s (%.2f%s)`, statetxt, diskuseflt, diskusesuffix))
	return template.HTML(s)
}

func (job *Job) Status() (ui jaws.HtmlGetter) {
	return uiJobStatus{job}
}
