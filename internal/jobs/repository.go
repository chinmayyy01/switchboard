package jobs

import (
	"context"
	"time"

	"github.com/chinmay/switchboard/internal/staff"
)

// TenantStore is the durable scheduling interface for one tenant. Booking
// implementations must return the original job when an idempotency key is
// retried and must never assign a technician to two booked jobs in one slot.
type TenantStore interface {
	SetWindows([]string)
	CurrentTime() time.Time
	AvailabilityContext(context.Context, string, string) ([]string, error)
	AlternativesContext(context.Context, string, string, int) ([]Slot, error)
	BookContext(context.Context, BookingRequest) (Job, error)
	ByPhoneContext(context.Context, string) ([]Job, error)
	GetContext(context.Context, string) (Job, bool, error)
	RescheduleContext(context.Context, string, string, string) error
	CancelContext(context.Context, string) error
	EmployeesContext(context.Context) ([]EmployeeWithJobs, error)
	SaveEmployeeContext(context.Context, staff.Employee) (staff.Employee, error)
	SetEmployeeActiveContext(context.Context, string, bool) error
}

type BookingRequest struct {
	IdempotencyKey string
	Service        string
	Date           string
	Window         string
	Address        string
	Phone          string
}

type EmployeeWithJobs struct {
	staff.Employee
	Busy []string `json:"busy"`
}
