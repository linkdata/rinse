package rinse

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/linkdata/jaws"
)

type uiTestPreview struct {
	*Rinse
	jaws.String
	Pages  jaws.Float
	Width  jaws.Float
	Iframe jaws.Bool
}

func (ui *uiTestPreview) JawsClick(e *jaws.Element, name string) (err error) {
	err = jaws.ErrEventUnhandled
	if name == "preview" {
		var req *http.Request
		q := url.Values{}
		if n := int(ui.Pages.Get()); n > 0 {
			q.Add("pages", strconv.Itoa(n))
		}
		if n := int(ui.Width.Get()); n > 0 {
			q.Add("width", strconv.Itoa(n))
		}
		if ui.Iframe.Get() {
			q.Set("iframe", "")
		}
		if req, err = http.NewRequest(http.MethodGet, ui.Config.ListenURL+"/preview/"+ui.Get()+"?"+q.Encode(), nil); err == nil {
			var resp *http.Response
			if resp, err = http.DefaultClient.Do(req); err == nil {
				toastResponse(ui.Jaws, resp)
			}
		}
	}
	return
}

func (ui *uiTestPreview) PagesParam() jaws.FloatSetter {
	return &ui.Pages
}

func (ui *uiTestPreview) WidthParam() jaws.FloatSetter {
	return &ui.Width
}

func (ui *uiTestPreview) IframeParam() jaws.BoolSetter {
	return &ui.Iframe
}

func (rns *Rinse) TestPreview() jaws.StringSetter {
	return &uiTestPreview{
		Rinse: rns,
		Pages: jaws.Float{Value: 1},
	}
}
