package rinser

import (
	"github.com/linkdata/jaws/lib/bind"
)

func (rns *Rinse) UiCleanupGotten() any {
	return bind.New(&rns.mu, &rns.cleanupGotten)
}
