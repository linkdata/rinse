package rinser

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func marshal(obj any) ([]byte, error) {
	if job, ok := obj.(*Job); ok {
		job.mu.Lock()
		defer job.mu.Unlock()
	}
	return json.Marshal(obj)
}

func HTTPJSON(hw http.ResponseWriter, code int, obj any) {
	if b, err := marshal(obj); err == nil {
		hw.Header().Set("Content-Type", "application/json")
		hw.WriteHeader(code)
		_, _ = hw.Write(b)
	} else {
		slog.Error("HTTPJSON", "err", err)
		hw.WriteHeader(http.StatusInternalServerError)
	}
}
