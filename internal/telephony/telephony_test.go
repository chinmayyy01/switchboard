package telephony

import (
	"testing"

	"github.com/chinmay/switchboard/internal/tenant"
)

func TestPlivoPlanUsesIndiaEndpointAndTenantMetadata(t *testing.T) {
	p := &Plivo{
		SIPSubdomain: "abc123",
		AgentName:    "switchboard-receptionist",
		SIPDomain:    "voice.plivo.com",
		Username:     "user",
		Password:     "pass",
	}
	plan, err := p.Plan(tenant.Tenant{
		ID:            "rivera",
		Name:          "Rivera",
		Telephony:     tenant.Telephony{Provider: "plivo", Number: "+911234567890"},
		ManagerNumber: "+919876543210",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.SIPEndpoint != "abc123.india.sip.livekit.cloud" {
		t.Fatalf("endpoint = %q", plan.SIPEndpoint)
	}
	if got := plan.InboundTrunk.Numbers[0]; got != "+911234567890" {
		t.Fatalf("number = %q", got)
	}
	if plan.DispatchRule.RoomConfig.Agents[0].AgentName != "switchboard-receptionist" {
		t.Fatalf("agent = %q", plan.DispatchRule.RoomConfig.Agents[0].AgentName)
	}
	if plan.DispatchRule.RoomConfig.Agents[0].Metadata != `{"tenant_id":"rivera"}` {
		t.Fatalf("metadata = %q", plan.DispatchRule.RoomConfig.Agents[0].Metadata)
	}
	if plan.OutboundTrunk == nil || plan.OutboundTrunk.DestinationCountry != "in" {
		t.Fatalf("outbound trunk = %+v", plan.OutboundTrunk)
	}
}

func TestPlivoPlanRequiresNumber(t *testing.T) {
	p := &Plivo{SIPSubdomain: "abc123", AgentName: "agent"}
	if _, err := p.Plan(tenant.Tenant{ID: "rivera"}); err == nil {
		t.Fatal("expected missing number error")
	}
}

func TestExotelIsExplicitlyUnavailable(t *testing.T) {
	if _, err := NewExotelFromEnv().Plan(tenant.Tenant{ID: "rivera"}); err == nil {
		t.Fatal("Exotel stub should not produce a plan")
	}
}
