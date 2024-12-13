package rinser

import (
	"github.com/linkdata/jaws"
)

func (rns *Rinse) UiCleanupGotten() any {
	return jaws.Bind(&rns.mu, &rns.cleanupGotten)
}
