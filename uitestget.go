package rinse

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"

	"github.com/linkdata/jaws"
)

type uiTestGet struct {
	*Rinse
	jaws.String
}

func toastResponse(jw *jaws.Jaws, resp *http.Response) {
	lvl := "success"
	if resp.StatusCode != http.StatusOK {
		lvl = "danger"
	}
	jw.Alert(lvl, describeResponse(resp))
}

func describeResponse(resp *http.Response) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n<br>", html.EscapeString(resp.Request.URL.String()))
	fmt.Fprintf(&sb, "%s\n<br>", html.EscapeString(resp.Status))
	for k, vv := range resp.Header {
		fmt.Fprintf(&sb, "%s: ", k)
		for i, v := range vv {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(html.EscapeString(v))
		}
		sb.WriteString("\n<br>")
	}
	sb.WriteString("\n<br>")
	if respBody, err := io.ReadAll(resp.Body); err == nil {
		if len(respBody) < 64 && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
			sb.WriteString(html.EscapeString(string(respBody)))
		} else if len(respBody) > 0 {
			fmt.Fprintf(&sb, "Body: %d bytes", len(respBody))
		}
	} else {
		fmt.Fprintf(&sb, "Body: Error: %s", html.EscapeString(err.Error()))
	}
	return sb.String()
}

func (u *uiTestGet) JawsClick(e *jaws.Element, name string) (err error) {
	err = jaws.ErrEventUnhandled
	if name == "get" {
		var req *http.Request
		if req, err = http.NewRequest(http.MethodGet, u.Config.ListenURL+"/job/"+u.Get(), nil); err == nil {
			var resp *http.Response
			if resp, err = http.DefaultClient.Do(req); err == nil {
				toastResponse(u.Jaws, resp)
			}
		}
	}
	return
}

func (rns *Rinse) TestGet() jaws.StringSetter {
	return &uiTestGet{
		Rinse: rns,
	}
}
