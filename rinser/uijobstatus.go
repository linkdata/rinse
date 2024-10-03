package rinser

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
	case JobExtractMeta:
		statetxt = "Extract Metadata"
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
func (ui uiJobStatus) JawsGetHtml(e *jaws.Element) template.HTML {
	ui.mu.Lock()
	diskuse := ui.Diskuse
	state := ui.state
	imgcount := len(ui.imgfiles)
	err := ui.Error
	errstate := ui.errstate
	var imgdone int
	for _, seen := range ui.imgfiles {
		if seen {
			imgdone++
		}
	}
	ui.Pages = imgcount
	ui.mu.Unlock()

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

	statetxt = html.EscapeString(statetxt)
	s := fmt.Sprintf(`<span class="%s">%s (%s)</span>`, stateclass, statetxt, prettyByteSize(diskuse))
	return template.HTML(s) // #nosec G203
}

func (job *Job) UiStatus() (ui jaws.HtmlGetter) {
	return uiJobStatus{job}
}
