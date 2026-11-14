package telephony

import (
	"fmt"

	"github.com/chinmay/switchboard/internal/tenant"
)

// Exotel is intentionally a stub. Inbound SIP routing, trial account limits,
// KYC, and SIP REFER support are still being confirmed, so the implementation
// must not be used in production yet.
type Exotel struct{}

func NewExotelFromEnv() *Exotel { return &Exotel{} }

func (e *Exotel) Name() string { return "exotel" }

func (e *Exotel) Plan(tenant.Tenant) (Plan, error) {
	return Plan{}, fmt.Errorf("%w: Exotel SIP routing and REFER support are awaiting confirmation", ErrNotConfigured)
}
