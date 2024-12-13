package rinser

import (
	"strings"

	"github.com/linkdata/jaws"
)

type uiAdmins struct {
	*Rinse
	v string
}

// JawsClick implements jaws.ClickHandler.
func (u *uiAdmins) JawsClick(e *jaws.Element, name string) (err error) {
	var adminlist []string
	for _, s1 := range strings.Split(u.v, ",") {
		for _, s2 := range strings.Split(s1, " ") {
			if s2 = strings.TrimSpace(s2); s2 != "" {
				adminlist = append(adminlist, s2)
			}
		}
	}
	if len(adminlist) > 0 {
		adminlist = append(adminlist, u.GetEmail(e.Initial()))
	}
	u.setAdmins(adminlist)
	u.v = strings.Join(u.getAdmins(), ", ")
	e.Dirty(u)
	return u.saveSettings()
}

func (u *uiAdmins) JawsSet(e *jaws.Element, v string) (err error) {
	u.v = v
	return
}

func (u *uiAdmins) JawsGet(e *jaws.Element) string {
	return u.v
}

func (rns *Rinse) UiAdmins() jaws.ClickHandler {
	return &uiAdmins{Rinse: rns, v: strings.Join(rns.getAdmins(), ", ")}
}
