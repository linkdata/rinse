package rinser

import (
	"github.com/linkdata/jaws"
)

type uiCleanupGotten struct{ *Rinse }

func (rns *Rinse) UiCleanupGotten() jaws.BoolSetter {
	return jaws.UiBool{L: &rns.mu, P: &rns.cleanupGotten}
}
