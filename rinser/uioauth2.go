package rinser

import (
	"encoding/json"

	"github.com/linkdata/jaws"
)

var uiOAuth2str string

type uiOAuth2 struct{ *Rinse }

func (ui uiOAuth2) JawsGetString(e *jaws.Element) (val string) {
	return uiOAuth2str
}

func (ui uiOAuth2) JawsSetString(e *jaws.Element, val string) (err error) {
	uiOAuth2str = val
	if newSettings := ui.validate([]byte(val)); newSettings != nil {
		e.Jaws.SetClass(ui, "is-valid")
		e.Jaws.RemoveClass(ui, "is-invalid")
		ui.SetOAuth2(newSettings)
		return ui.saveSettings()
	} else {
		e.Jaws.SetClass(ui, "is-invalid")
		e.Jaws.RemoveClass(ui, "is-valid")
	}
	return nil
}

func (ui uiOAuth2) validate(val []byte) (newSettings *OAuth2Settings) {
	var obj OAuth2Settings
	if err := json.Unmarshal(val, &obj); err == nil && obj.Valid() {
		newSettings = &obj
	}
	return
}

func (rns *Rinse) UiOAuth2() jaws.StringSetter {
	if b, err := json.MarshalIndent(rns.OAuth2Settings, "", "  "); err == nil {
		uiOAuth2str = string(b)
	}
	return uiOAuth2{rns}
}
