package rinser

import (
	"encoding/json"
	"io"
	"net/http"
)

func ctxShouldBindJSON(hr *http.Request, obj any) (err error) {
	var b []byte
	if b, err = io.ReadAll(hr.Body); err == nil {
		err = json.Unmarshal(b, obj)
	}
	return
}
