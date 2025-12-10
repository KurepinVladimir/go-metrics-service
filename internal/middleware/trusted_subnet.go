package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ValidateTrustedSubnet пропускает запросы только от IP-адресов,
// попадающих в trustedNet. Если trustedNet == nil, мидлварь
// не выполняет никаких проверок.
func ValidateTrustedSubnet(trustedNet *net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Если подсеть не задана — просто пропускаем запросы
		if trustedNet == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ipStr := strings.TrimSpace(r.Header.Get("X-Real-IP"))
			ip := net.ParseIP(ipStr)

			// Отсеиваем пустые/некорректные IP или запросы вне доверенной подсети
			if ip == nil || !trustedNet.Contains(ip) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
