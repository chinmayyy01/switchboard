package telephony

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/chinmay/switchboard/internal/tenant"
)

// Plivo is the primary telephony provider. It uses Plivo as the SIP trunk
// carrier and LiveKit SIP for the media path. Credentials stay in the
// environment.
type Plivo struct {
	SIPSubdomain  string
	AgentName     string
	SIPDomain     string
	Username      string
	Password      string
	DefaultNumber string
}

func NewPlivoFromEnv() (*Plivo, error) {
	subdomain := strings.TrimSpace(os.Getenv("LIVEKIT_SIP_SUBDOMAIN"))
	if subdomain == "" {
		return nil, fmt.Errorf("%w: LIVEKIT_SIP_SUBDOMAIN is required", ErrNotConfigured)
	}
	agentName := strings.TrimSpace(os.Getenv("LIVEKIT_AGENT_NAME"))
	if agentName == "" {
		agentName = "switchboard-receptionist"
	}
	return &Plivo{
		SIPSubdomain:  subdomain,
		AgentName:     agentName,
		SIPDomain:     strings.TrimSpace(os.Getenv("PLIVO_SIP_DOMAIN")),
		Username:      strings.TrimSpace(os.Getenv("PLIVO_USERNAME")),
		Password:      strings.TrimSpace(os.Getenv("PLIVO_PASSWORD")),
		DefaultNumber: strings.TrimSpace(os.Getenv("PLIVO_DEFAULT_NUMBER")),
	}, nil
}

func (p *Plivo) Name() string { return "plivo" }

func (p *Plivo) Plan(t tenant.Tenant) (Plan, error) {
	if strings.ToLower(strings.TrimSpace(t.Telephony.Provider)) != "" &&
		strings.ToLower(strings.TrimSpace(t.Telephony.Provider)) != "plivo" {
		return Plan{}, fmt.Errorf("tenant %s uses telephony provider %q", t.ID, t.Telephony.Provider)
	}
	number := firstNonEmpty(t.Telephony.Number, p.DefaultNumber)
	if number == "" {
		return Plan{}, fmt.Errorf("tenant %s has no phone number", t.ID)
	}
	metadata, err := json.Marshal(map[string]string{"tenant_id": t.ID})
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{
		TenantID:    t.ID,
		Provider:    p.Name(),
		SIPEndpoint: fmt.Sprintf("%s.india.sip.livekit.cloud", p.SIPSubdomain),
		InboundTrunk: InboundTrunk{
			Name:         fmt.Sprintf("sb-%s-inbound", t.ID),
			Numbers:      []string{number},
			KrispEnabled: true,
		},
		DispatchRule: DispatchRule{
			Name: fmt.Sprintf("sb-%s-dispatch", t.ID),
			Rule: Rule{Individual: &Individual{RoomPrefix: "call-"}},
			RoomConfig: RoomConfig{Agents: []AgentDispatch{{
				AgentName: p.AgentName,
				Metadata:  string(metadata),
			}}},
		},
	}
	if t.ManagerPhone() != "" {
		if p.SIPDomain == "" {
			return Plan{}, errors.New("PLIVO_SIP_DOMAIN is required for warm transfer")
		}
		plan.OutboundTrunk = &OutboundTrunk{
			Name:               fmt.Sprintf("sb-%s-outbound", t.ID),
			Address:            p.SIPDomain,
			Numbers:            []string{number},
			AuthUsername:       p.Username,
			AuthPassword:       p.Password,
			DestinationCountry: "in",
		}
	}
	return plan, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
