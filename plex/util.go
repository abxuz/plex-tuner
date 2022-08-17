package plex

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

func getBaseUrl(r Request) string {
	var scheme string
	if r.TLS == nil {
		scheme = "http"
	} else {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func internalServerError(w ResponseWriter, err string) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err))
}

func writeJson(w ResponseWriter, obj any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}

func allowCORS(w ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Acccess-Control-Allow-Credentials", "true")
}

func disableCache(w ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
}

func disableKeepalive(w ResponseWriter) {
	h := w.Header()
	h.Set("Connection", "close")
}

func isWebsocketUpgrade(r Request) bool {
	connection := r.Header.Get("Connection")
	connection = strings.ToLower(connection)
	return connection == "upgrade"
}

func isPlex(r Request) bool {
	return strings.HasPrefix(r.UserAgent(), "Lavf/")
}

func getContent(p string) ([]byte, error) {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		resp, err := httpClient.Get(p)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		return data, err
	}
	return os.ReadFile(p)
}
