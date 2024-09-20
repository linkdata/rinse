package rinse

import (
	"net/http"
)

type HTTPError struct {
	Code  int
	Error string
}

func SendHTTPError(hw http.ResponseWriter, code int, err error) {
	var txt string
	if err != nil {
		txt = err.Error()
	} else {
		txt = http.StatusText(code)
	}
	herr := HTTPError{
		Code:  code,
		Error: txt,
	}
	HTTPJSON(hw, code, herr)
}
