package payment

import (
	"strings"
	"time"

	"beta-go/api/internal/order"
)

// Decision is the next payment and order state after a provider result.
type Decision struct {
	PaymentStatus string
	OrderStatus   string
	Late          bool
	Reason        string
	Changed       bool
}

// State is the locked payment and order at the moment a result arrives.
type State struct {
	PaymentStatus string
	OrderStatus   string
	ExpiresAt     time.Time
}

// decide maps a provider result onto payment and order statuses.
// A client claim of success never reaches this function.
func decide(state State, providerStatus string, at time.Time) Decision {
	providerStatus = strings.ToUpper(strings.TrimSpace(providerStatus))
	expired := !state.ExpiresAt.IsZero() && !at.Before(state.ExpiresAt)
	switch providerStatus {
	case ProviderSuccess:
		return decideSuccess(state, expired)
	case ProviderFailed:
		return decideFailed(state, expired)
	case ProviderPending:
		if expired && state.PaymentStatus == StatusPending && state.OrderStatus == order.StatusPendingPayment {
			return Decision{
				PaymentStatus: StatusExpired,
				OrderStatus:   order.StatusExpired,
				Reason:        ReasonUnpaidExpired,
				Changed:       true,
			}
		}
	}
	return Decision{PaymentStatus: state.PaymentStatus, OrderStatus: state.OrderStatus}
}

func decideSuccess(state State, expired bool) Decision {
	switch state.PaymentStatus {
	case StatusPaid, StatusRefundPending, StatusRefunded, StatusRefundFailed:
		return Decision{PaymentStatus: state.PaymentStatus, OrderStatus: state.OrderStatus}
	}
	if state.OrderStatus == order.StatusPendingPayment && !expired {
		return Decision{
			PaymentStatus: StatusPaid,
			OrderStatus:   order.StatusAwaitingVendor,
			Reason:        ReasonProviderSuccess,
			Changed:       true,
		}
	}
	if state.OrderStatus == order.StatusCancelled || state.OrderStatus == order.StatusExpired || expired {
		return Decision{
			PaymentStatus: StatusRefundPending,
			OrderStatus:   orderStatusAfterLate(state.OrderStatus),
			Late:          true,
			Reason:        ReasonLatePayment,
			Changed:       true,
		}
	}
	if state.PaymentStatus != StatusPaid {
		return Decision{
			PaymentStatus: StatusPaid,
			OrderStatus:   state.OrderStatus,
			Reason:        ReasonProviderSuccess,
			Changed:       true,
		}
	}
	return Decision{PaymentStatus: state.PaymentStatus, OrderStatus: state.OrderStatus}
}

func decideFailed(state State, expired bool) Decision {
	switch state.PaymentStatus {
	case StatusPaid, StatusRefundPending, StatusRefunded, StatusRefundFailed, StatusExpired:
		return Decision{PaymentStatus: state.PaymentStatus, OrderStatus: state.OrderStatus}
	}
	if expired && state.OrderStatus == order.StatusPendingPayment {
		return Decision{
			PaymentStatus: StatusExpired,
			OrderStatus:   order.StatusExpired,
			Reason:        ReasonUnpaidExpired,
			Changed:       true,
		}
	}
	if state.PaymentStatus == StatusPending && state.OrderStatus == order.StatusPendingPayment {
		return Decision{
			PaymentStatus: StatusFailed,
			OrderStatus:   state.OrderStatus,
			Reason:        "PROVIDER_FAILED",
			Changed:       true,
		}
	}
	return Decision{PaymentStatus: state.PaymentStatus, OrderStatus: state.OrderStatus}
}

func orderStatusAfterLate(current string) string {
	if current == order.StatusPendingPayment {
		return order.StatusExpired
	}
	return current
}
