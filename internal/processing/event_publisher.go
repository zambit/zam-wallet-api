package processing

import (
	"context"
	"git.zam.io/wallet-backend/web-api/pkg/services/broker"
)

// Publisher part of broker interface
type Publisher interface {
	PublishCtx(ctx context.Context, identifier broker.Identifier, payload interface{}) error
}


