package rinser

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/linkdata/jaws"
)

type uiExternalIP struct{ *Rinse }

// JawsGetHtml implements jaws.HtmlGetter.
func (u uiExternalIP) JawsGetHtml(e *jaws.Element) (s template.HTML) {
	u.mu.Lock()
	s = u.externalIP
	u.mu.Unlock()
	return
}

func (rns *Rinse) UiExternalIP() (ui jaws.HtmlGetter) {
	return uiExternalIP{rns}
}

func (rns *Rinse) UpdateExternalIP() {
	publicip, err := rns.GetExternalIP()
	rns.mu.Lock()
	defer rns.mu.Unlock()
	if err != nil {
		publicip = fmt.Sprintf(`<span class="text-danger" data-toggle="tooltip" title="%s">unknown</span>`, html.EscapeString(err.Error()))
	}
	rns.externalIP = template.HTML(publicip) //#nosec G203
	rns.Jaws.Dirty(rns.UiExternalIP())
}

func (rns *Rinse) GetExternalIP() (publicip string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var req *http.Request
	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://am.i.mullvad.net/ip", nil); err == nil {
		var resp *http.Response
		if resp, err = rns.getClient().Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if body, err := io.ReadAll(resp.Body); err == nil {
					publicip = string(body)
				}
			} else {
				_, _ = io.Copy(io.Discard, resp.Body)
			}
		}
		if i := strings.LastIndexByte(publicip, ':'); i >= 0 {
			publicip = publicip[:i]
		}
		publicip = strings.TrimSpace(publicip)
	}
	return
}
