package processing

import "time"

// Scheme holds configuration values for processing module
type Scheme struct {
	// TimeToWaitRecipient time before cancelling tx, which awaits wallet creation of recipient
	//
	// Default: 72h
	TimeToWaitRecipient time.Duration
}
