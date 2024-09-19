package rinse

import (
	"net/http"

	"github.com/linkdata/jaws"
)

type uiTestDelete struct {
	*Rinse
	jaws.String
}

func (u *uiTestDelete) JawsClick(e *jaws.Element, name string) (err error) {
	err = jaws.ErrEventUnhandled
	if name == "delete" {
		var req *http.Request
		if req, err = http.NewRequest(http.MethodDelete, u.Config.ListenURL+"/job/"+u.Get(), nil); err == nil {
			var resp *http.Response
			if resp, err = http.DefaultClient.Do(req); err == nil {
				toastResponse(u.Jaws, resp)
			}
		}
	}
	return
}

func (rns *Rinse) TestDelete() jaws.StringSetter {
	return &uiTestDelete{
		Rinse: rns,
	}
}
