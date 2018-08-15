package isc

import "github.com/ericlagergren/decimal"

// TxEventPayload describes tx details
type TxEventPayload struct {
	Coin           string
	Type           string
	FromPhone      string
	FromWalletName string
	ToPhone        string
	ToAddress      string
	Amount         *decimal.Big
}

// ITxsEventNotificator used to notify other system parts about events occurred while processing transaction
type ITxsEventNotificator interface {
	// Processed notify that tx has been successfully processed
	Processed(payload TxEventPayload) error

	// Declined notify tx has been declined due to decline reason
	Declined(payload TxEventPayload, declineReason error) error

	// AwaitRecipient notify that tx awaits recipient
	AwaitRecipient(payload TxEventPayload) error
}
