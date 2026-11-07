package telephony

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/chinmay/switchboard/internal/tenant"
)

var ErrNotConfigured = errors.New("telephony provider not configured")

// Plan is the reviewed, provider-specific configuration needed to attach a
// tenant's number to LiveKit. It is emitted as JSON for the LiveKit CLI so
// provisioning stays explicit and never happens implicitly at startup.
type Plan struct {
	TenantID      string         `json:"tenant_id"`
	Provider      string         `json:"provider"`
	SIPEndpoint   string         `json:"sip_endpoint"`
	InboundTrunk  InboundTrunk   `json:"inbound_trunk"`
	DispatchRule  DispatchRule   `json:"dispatch_rule"`
	OutboundTrunk *OutboundTrunk `json:"outbound_trunk,omitempty"`
}

type InboundTrunk struct {
	Name         string   `json:"name"`
	Numbers      []string `json:"numbers"`
	KrispEnabled bool     `json:"krisp_enabled,omitempty"`
}

type DispatchRule struct {
	Name       string     `json:"name"`
	Rule       Rule       `json:"rule"`
	RoomConfig RoomConfig `json:"room_config"`
}

type Rule struct {
	Individual *Individual `json:"dispatch_rule_individual,omitempty"`
}

type Individual struct {
	RoomPrefix string `json:"room_prefix"`
}

type RoomConfig struct {
	Agents []AgentDispatch `json:"agents"`
}

type AgentDispatch struct {
	AgentName string `json:"agent_name"`
	Metadata  string `json:"metadata,omitempty"`
}

type OutboundTrunk struct {
	Name               string   `json:"name"`
	Address            string   `json:"address"`
	Numbers            []string `json:"numbers"`
	AuthUsername       string   `json:"auth_username,omitempty"`
	AuthPassword       string   `json:"auth_password,omitempty"`
	DestinationCountry string   `json:"destination_country,omitempty"`
}

type Provider interface {
	Name() string
	Plan(tenant.Tenant) (Plan, error)
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(providers ...Provider) *Registry {
	byName := make(map[string]Provider, len(providers))
	for _, provider := range providers {
		byName[provider.Name()] = provider
	}
	return &Registry{providers: byName}
}

func (r *Registry) Get(name string) (Provider, bool) {
	provider, ok := r.providers[strings.ToLower(strings.TrimSpace(name))]
	return provider, ok
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.providers))
	for name := range r.providers {
		out = append(out, name)
	}
	return out
}

// FromEnv selects the telephony provider. Plivo is the primary provider; Exotel
// is present behind the same interface but returns ErrNotConfigured until its
// routing and REFER support are confirmed.
func FromEnv() (Provider, error) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("TELEPHONY_PROVIDER")))
	if name == "" {
		name = "plivo"
	}
	switch name {
	case "plivo":
		return NewPlivoFromEnv()
	case "exotel":
		return NewExotelFromEnv(), nil
	default:
		return nil, fmt.Errorf("unknown telephony provider %q", name)
	}
}

func sipEndpoint() string {
	subdomain := strings.TrimSpace(os.Getenv("LIVEKIT_SIP_SUBDOMAIN"))
	if subdomain == "" {
		return ""
	}
	return fmt.Sprintf("%s.india.sip.livekit.cloud", subdomain)
}
