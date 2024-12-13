package rinser

import (
	"net/url"
	"sync"

	"github.com/linkdata/jaws"
)

type uiProxy struct {
	*Rinse
	jaws.Binder[string]
}

type uiProxyButton struct{ *uiProxy }

func (ui uiProxyButton) JawsClick(e *jaws.Element, name string) (err error) {
	urlStr := ui.Binder.JawsGet(e)
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

func (u *uiProxy) Address() any {
	return u.Binder
}

func (u *uiProxy) ExternalIP() jaws.HtmlGetter {
	return u.UiExternalIP()
}

func (u *uiProxy) Button() jaws.ClickHandler {
	return uiProxyButton{u}
}

func (rns *Rinse) UiProxy() *uiProxy {
	address := rns.ProxyURL()
	var mu sync.Mutex
	return &uiProxy{Rinse: rns, Binder: jaws.Bind(&mu, &address)}
}
