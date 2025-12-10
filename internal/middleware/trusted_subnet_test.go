package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTrustedSubnet(t *testing.T) {
	_, network, _ := net.ParseCIDR("192.168.1.0/24")
	mw := ValidateTrustedSubnet(network)

	okHandler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqAllowed := httptest.NewRequest(http.MethodPost, "/", nil)
	reqAllowed.Header.Set("X-Real-IP", "192.168.1.10")
	respAllowed := httptest.NewRecorder()
	okHandler.ServeHTTP(respAllowed, reqAllowed)
	assert.Equal(t, http.StatusOK, respAllowed.Code)

	reqDenied := httptest.NewRequest(http.MethodPost, "/", nil)
	reqDenied.Header.Set("X-Real-IP", "10.0.0.1")
	respDenied := httptest.NewRecorder()
	okHandler.ServeHTTP(respDenied, reqDenied)
	assert.Equal(t, http.StatusForbidden, respDenied.Code)

	reqInvalid := httptest.NewRequest(http.MethodPost, "/", nil)
	reqInvalid.Header.Set("X-Real-IP", "not-an-ip")
	respInvalid := httptest.NewRecorder()
	okHandler.ServeHTTP(respInvalid, reqInvalid)
	assert.Equal(t, http.StatusForbidden, respInvalid.Code)
}

func TestValidateTrustedSubnet_Disabled(t *testing.T) {
	mw := ValidateTrustedSubnet(nil)
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Real-IP", "203.0.113.5")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusNoContent, resp.Code)
}
