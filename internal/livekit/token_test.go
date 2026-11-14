package livekit

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMintAndVerify(t *testing.T) {
	token, err := Mint(Options{
		APIKey:    "key",
		APISecret: "secret",
		Identity:  "caller-1",
		Name:      "Caller",
		Metadata:  `{"tenant_id":"rivera"}`,
		TTL:       time.Minute,
		Grants: Grants{
			Room:           "call-123",
			RoomJoin:       true,
			CanPublish:     true,
			CanSubscribe:   true,
			CanPublishData: true,
		},
		RoomConfig: &RoomConfiguration{
			Agents: []RoomAgentDispatch{{
				AgentName: "switchboard-receptionist",
				Metadata:  `{"tenant_id":"rivera"}`,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := Verify("secret", token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "caller-1" || claims.Video.Room != "call-123" || !claims.Video.RoomJoin {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.RoomConfig == nil || len(claims.RoomConfig.Agents) != 1 {
		t.Fatalf("agent dispatch missing: %+v", claims.RoomConfig)
	}
	if claims.RoomConfig.Agents[0].AgentName != "switchboard-receptionist" {
		t.Fatalf("agent name = %q", claims.RoomConfig.Agents[0].AgentName)
	}

	parts := strings.Split(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["iss"] != "key" {
		t.Fatalf("issuer = %v", decoded["iss"])
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	token, err := Mint(Options{APIKey: "key", APISecret: "secret", Identity: "a", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"attacker","exp":9999999999,"video":{}}`))
	if _, err := Verify("secret", strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered token accepted")
	}
}
