package rinser

import (
	"net/url"

	"github.com/linkdata/jaws"
)

type uiProxy struct{ *Rinse }

// JawsGetString implements jaws.StringSetter.
func (u uiProxy) JawsGetString(e *jaws.Element) (s string) {
	u.mu.Lock()
	s = u.proxyUrl
	u.mu.Unlock()
	return
}

// JawsSetString implements jaws.StringSetter.
func (ui uiProxy) JawsSetString(e *jaws.Element, v string) (err error) {
	ui.mu.Lock()
	ui.proxyUrl = v
	ui.mu.Unlock()
	var u *url.URL
	if u, err = url.Parse(v); err == nil {
		if u.Scheme != "" && u.Host != "" {
			go ui.UpdateExternalIP()
			err = ui.saveSettings()
		}
	}
	return
}

func (rns *Rinse) UiProxy() jaws.StringSetter {
	return uiProxy{rns}
}
