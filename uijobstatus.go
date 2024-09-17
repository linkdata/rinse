package rinse

import (
	"fmt"
	"html"
	"html/template"
	"strconv"

	"github.com/linkdata/jaws"
)

type uiJobStatus struct{ *Job }

func jobStateText(n JobState) (statetxt string) {
	switch n {
	case JobNew:
		statetxt = "Waiting"
	case JobStarting:
		statetxt = "Starting"
	case JobDownload:
		statetxt = "Downloading"
	case JobDetectLanguage:
		statetxt = "Detect Language"
	case JobDocToPdf:
		statetxt = "Converting"
	case JobPdfToImages:
		statetxt = "Rendering"
	case JobTesseract:
		statetxt = "Scanning"
	case JobEnding:
		statetxt = "Cleanup"
	case JobFinished:
		statetxt = "Finished"
	case JobFailed:
		statetxt = "Failed"
	default:
		statetxt = strconv.Itoa(int(n))
	}
	return
}

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiJobStatus) JawsGetHtml(e *jaws.Element) template.HTML {
	u.mu.Lock()
	diskuse := u.diskuse
	state := u.state
	imgcount := len(u.imgfiles)
	err := u.err
	errstate := u.errstate
	var imgdone int
	for _, seen := range u.imgfiles {
		if seen {
			imgdone++
		}
	}
	u.mu.Unlock()

	statetxt := jobStateText(state)
	stateclass := "text-body"
	switch state {
	case JobNew:
		stateclass = "text-secondary fw-light"
	case JobPdfToImages:
		statetxt = fmt.Sprintf("Rendered %d", imgcount)
	case JobTesseract:
		statetxt = fmt.Sprintf("Scanning %d/%d", imgdone, imgcount)
	case JobFinished:
		stateclass = "text-success fw-bold"
	case JobFailed:
		stateclass = "text-danger fw-bold"
		statetxt = jobStateText(errstate)
		if err != nil {
			statetxt += fmt.Sprintf(": %v", err)
		}
	}
	s := fmt.Sprintf(`<span class="%s">%s (%s)</span>`, stateclass, html.EscapeString(statetxt), prettyByteSize(diskuse))
	return template.HTML(s) // #nosec G203
}

func (job *Job) Status() (ui jaws.HtmlGetter) {
	return uiJobStatus{job}
}
