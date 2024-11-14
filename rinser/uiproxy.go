package rinser

import (
	"github.com/linkdata/jaws"
)

type uiProxy struct {
	*Rinse
	jaws.String
}

type uiProxyButton struct{ *uiProxy }

func (ui uiProxyButton) JawsClick(e *jaws.Element, name string) (err error) {
	ui.mu.Lock()
	ui.proxyUrl = ui.String.Get()
	ui.mu.Unlock()
	go ui.UpdateExternalIP()
	return ui.saveSettings()
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
