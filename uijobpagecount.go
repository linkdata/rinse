package rinse

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"

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
		diskuse = 0
		filepath.WalkDir(u.Workdir, func(path string, d fs.DirEntry, err error) error {
			if fi, err := d.Info(); err == nil {
				diskuse += fi.Size()
			}
			if filepath.Ext(d.Name()) == ".ppm" {
				todo++
			}
			return nil
		})
		u.mu.Lock()
		u.diskuse = diskuse
		u.mu.Unlock()
	}
	return template.HTML(fmt.Sprintf(`<span data-toggle="tooltip" title="%dMB">%d/%d</span>`, diskuse/(1024*1024), done, todo+done))
}

func (job *Job) Pagecount() (ui jaws.HtmlGetter) {
	return uiJobPagecount{job}
}
