package rinser

import (
	"net/url"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type uiProxy struct {
	*Rinse
	bind.Binder[string]
}

type uiProxyButton struct{ *uiProxy }

func (ui uiProxyButton) JawsClick(e *jaws.Element, _ jaws.Click) (err error) {
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

func (u *uiProxy) ExternalIP() bind.HTMLGetter {
	return u.UiExternalIP()
}

func (u *uiProxy) Button() jaws.ClickHandler {
	return uiProxyButton{u}
}

func (rns *Rinse) UiProxy() *uiProxy {
	address := rns.ProxyURL()
	var mu sync.Mutex
	return &uiProxy{Rinse: rns, Binder: bind.New(&mu, &address)}
}
