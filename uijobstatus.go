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
	err := u.err
	var ppmdone int
	for _, seen := range u.ppmfiles {
		if seen {
			ppmdone++
		}
	}
	u.mu.Unlock()

	var statetxt string
	stateclass := "text-body"
	switch state {
	case JobNew:
		statetxt = "Waiting"
		stateclass = "text-secondary fw-light"
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
		stateclass = "text-success fw-bold"
	case JobFailed:
		statetxt = "Failed"
		stateclass = "text-danger fw-bold"
		if err != nil {
			statetxt = err.Error()
		}
	}
	s := fmt.Sprintf(`<span class="%s">%s (%s)</span>`, stateclass, html.EscapeString(statetxt), prettyByteSize(diskuse))
	return template.HTML(s) // #nosec G203
}

func (job *Job) Status() (ui jaws.HtmlGetter) {
	return uiJobStatus{job}
}
