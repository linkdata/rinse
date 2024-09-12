package rinse

import (
	"bytes"
	"net/http"

	"github.com/linkdata/jaws"
)

type uiTestPut struct {
	*Rinse
	jaws.String
}

func (u *uiTestPut) JawsClick(e *jaws.Element, name string) (err error) {
	err = jaws.ErrEventUnhandled
	if name == "put" {
		reqBody := bytes.NewBufferString("Hello World!")
		var req *http.Request
		q := u.Get()
		if q == "" {
			q = "filename.txt?lang=eng"
		}
		if req, err = http.NewRequest(http.MethodPut, u.Config.ListenURL+"/job/"+q, reqBody); err == nil {
			var resp *http.Response
			if resp, err = http.DefaultClient.Do(req); err == nil {
				toastResponse(u.Jaws, resp)
			}
		}
	}
	return
}

func (rns *Rinse) TestPut() jaws.StringSetter {
	return &uiTestPut{
		Rinse: rns,
	}
}
