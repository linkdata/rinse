package rinser

import (
	"net/url"

	"github.com/linkdata/jaws"
)

type uiProxy struct {
	*Rinse
	jaws.String
}

type uiProxyButton struct{ *uiProxy }

func (ui uiProxyButton) JawsClick(e *jaws.Element, name string) (err error) {
	urlStr := ui.String.Get()
	if urlStr != "" {
		var u *url.URL
		if u, err = url.Parse(urlStr); err == nil {
			if u.Scheme != "socks5h" {
				e.Alert("warning", "Proxy scheme is not <code>socks5h</code>")
			}
		}
	}
	if err == nil {
		ui.mu.Lock()
		ui.proxyUrl = urlStr
		ui.mu.Unlock()
		go ui.UpdateExternalIP()
		err = ui.saveSettings()
	}
	return
}

func (u *uiProxy) Address() jaws.StringSetter {
	return &u.String
}

func (u *uiProxy) ExternalIP() jaws.HtmlGetter {
	return u.UiExternalIP()
}

func (u *uiProxy) Button() jaws.ClickHandler {
	return uiProxyButton{u}
}

func (rns *Rinse) UiProxy() *uiProxy {
	return &uiProxy{Rinse: rns, String: jaws.String{Value: rns.ProxyURL()}}
}
