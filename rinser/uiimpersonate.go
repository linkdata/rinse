package rinser

import (
	"strings"

	"github.com/linkdata/jaws"
)

type uiImpersonate struct {
	*Rinse
	v string
}

func (u *uiImpersonate) JawsClick(e *jaws.Element, name string) (err error) {
	e.Session().Set(u.JawsAuth.SessionEmailKey, strings.TrimSpace(u.v))
	return
}

func (u *uiImpersonate) JawsSetString(e *jaws.Element, v string) (err error) {
	u.v = v
	return
}

func (u *uiImpersonate) JawsGetString(e *jaws.Element) string {
	return u.v
}

func (rns *Rinse) UiImpersonate() jaws.ClickHandler {
	return &uiImpersonate{Rinse: rns}
}
