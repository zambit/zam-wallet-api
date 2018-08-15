package simple

import (
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	"git.zam.io/wallet-backend/web-api/pkg/services/notifications"
	"github.com/pkg/errors"
	"strings"
)

// txsEventNotificator simply renders user notification in simple text form and uses notifications.ITransport to send
// them. This implementation doesn't actually sends broker messages.
type txsEventNotificator struct {
	transport notifications.ITransport
}

// New simple ITxEventsNotificator implementation which uses specified message transport
func New(transport notifications.ITransport) isc.ITxsEventNotificator {
	return &txsEventNotificator{transport}
}

// Processed implements ITxEventNotificator
func (*txsEventNotificator) Processed(payload isc.TxEventPayload) error {
	// does nothing right now
	return nil
}

// Declined implements ITxEventNotificator
func (*txsEventNotificator) Declined(payload isc.TxEventPayload, declineReason error) error {
	// does nothing right now
	return nil
}

// AwaitRecipient implements ITxEventNotificator
func (n *txsEventNotificator) AwaitRecipient(payload isc.TxEventPayload) error {
	if payload.ToPhone == "" {
		return errors.New("simple txs event notificator: empty ToPhone field")
	}
	if payload.Coin == "" {
		return errors.New("simple txs event notificator: empty ToCoin field")
	}
	if payload.FromPhone == "" {
		return errors.New("simple txs event notificator: empty FromPhone field")
	}
	if payload.Amount == nil {
		return errors.New("simple txs event notificator: empty Amount field")
	}

	return n.transport.Send(
		payload.ToPhone,
		fmt.Sprintf(
			`%s

отправил вам

%s %s

. Для их получения зарегестрируйтесь на http://wallet-test.zam.io в течении 72х часов`,
			payload.FromPhone, payload.Amount.String(), strings.ToUpper(payload.Coin),
		),
	)
}
