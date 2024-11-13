package rinser

import "github.com/linkdata/jaws"

type uiProxy struct{ *Rinse }

func (rns *Rinse) UiProxy() jaws.StringSetter {
	return jaws.UiString{L: &rns.mu, P: &rns.proxyUrl}
}
