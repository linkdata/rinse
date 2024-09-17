package rinse

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/linkdata/jaws"
)

type uiTestPut struct {
	*Rinse
	File jaws.String
	URL  jaws.String
	Lang jaws.String
}

func (ui *uiTestPut) JawsClick(e *jaws.Element, name string) (err error) {
	err = jaws.ErrEventUnhandled
	if name == "put" {
		var reqBody io.Reader
		var req *http.Request
		if ui.File.Get() != "" {
			reqBody = bytes.NewBufferString("Hello World!")
		}
		dstUrl := fmt.Sprintf("%s/job?url=%s&file=%s&lang=%s",
			ui.Config.ListenURL,
			url.QueryEscape(ui.URL.Get()),
			url.QueryEscape(ui.File.Get()),
			url.QueryEscape(ui.Lang.Get()),
		)
		if req, err = http.NewRequest(http.MethodPut, dstUrl, reqBody); err == nil {
			var resp *http.Response
			if resp, err = http.DefaultClient.Do(req); err == nil {
				toastResponse(ui.Jaws, resp)
			}
		}
	}
	return
}

func (ui *uiTestPut) FileParam() jaws.StringSetter {
	return &ui.File
}

func (ui *uiTestPut) URLParam() jaws.StringSetter {
	return &ui.URL
}

func (ui *uiTestPut) LangParam() jaws.StringSetter {
	return &ui.Lang
}

func (rns *Rinse) TestPut() jaws.ClickHandler {
	return &uiTestPut{
		Rinse: rns,
	}
}
