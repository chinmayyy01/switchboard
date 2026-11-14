package api

import "testing"

func TestCurrentPhoneStatus(t *testing.T) {
	t.Setenv("LIVEKIT_SIP_NUMBER", "")
	t.Setenv("PLIVO_DEFAULT_NUMBER", "")
	t.Setenv("LIVEKIT_SIP_SUBDOMAIN", "")
	t.Setenv("PLIVO_SIP_DOMAIN", "")
	if got := currentPhoneStatus(); got.Connected || got.Number != "" || got.Provider != "" {
		t.Fatalf("empty env = %+v", got)
	}

	t.Setenv("LIVEKIT_SIP_SUBDOMAIN", "switchboard.sip.livekit.cloud")
	t.Setenv("LIVEKIT_SIP_NUMBER", "+911140000000")
	got := currentPhoneStatus()
	if !got.Connected || got.Number != "+911140000000" || got.Provider != "livekit" {
		t.Fatalf("livekit = %+v", got)
	}

	t.Setenv("LIVEKIT_SIP_SUBDOMAIN", "")
	t.Setenv("LIVEKIT_SIP_NUMBER", "")
	t.Setenv("PLIVO_SIP_DOMAIN", "sip.plivo.com")
	t.Setenv("PLIVO_DEFAULT_NUMBER", "+919900000000")
	got = currentPhoneStatus()
	if !got.Connected || got.Provider != "plivo" || got.Number != "+919900000000" {
		t.Fatalf("plivo = %+v", got)
	}

	// A number without a carrier domain is not a working line.
	t.Setenv("PLIVO_SIP_DOMAIN", "")
	if got := currentPhoneStatus(); got.Connected {
		t.Fatalf("number without domain reported connected: %+v", got)
	}
}
