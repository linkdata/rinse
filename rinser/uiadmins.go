package rinser

import (
	"sort"
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
	if email, ok := e.Session().Get(u.JawsAuth.SessionEmailKey).(string); ok {
		adminlist = append(adminlist, email)
	}
	u.setAdmins(adminlist)
	u.v = strings.Join(u.getAdmins(), ", ")
	return u.saveSettings()
}

func (rns *Rinse) getAdminsLocked() (v []string) {
	for k := range rns.admins {
		v = append(v, k)
	}
	sort.Strings(v)
	return
}

func (rns *Rinse) getAdmins() (v []string) {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	return rns.getAdminsLocked()
}

func (rns *Rinse) setAdminsLocked(v []string) {
	sort.Strings(v)
	clear(rns.admins)
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			rns.admins[s] = struct{}{}
		}
	}
}

func (rns *Rinse) setAdmins(v []string) {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	rns.setAdminsLocked(v)
}

func (u *uiAdmins) JawsSetString(e *jaws.Element, v string) (err error) {
	u.v = v
	return
}

func (u *uiAdmins) JawsGetString(e *jaws.Element) string {
	return u.v
}

func (rns *Rinse) UiAdmins() jaws.ClickHandler {
	return &uiAdmins{Rinse: rns, v: strings.Join(rns.getAdmins(), ", ")}
}
