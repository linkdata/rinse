package rinser

import (
	"github.com/linkdata/jaws"
)

type uiUser struct{}

func (ui uiUser) JawsGetString(e *jaws.Element) string {
	if usr, ok := e.Session().Get("user").(string); ok {
		return usr
	}
	return ""
}

func (rns *Rinse) UiUser() jaws.StringGetter {
	return uiUser{}
}
