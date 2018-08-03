package base

import (
	"git.zam.io/wallet-backend/common/pkg/errors"
	"git.zam.io/wallet-backend/web-api/pkg/services/broker"
	"github.com/segmentio/objconv/json"
)

// HandlerOutItem
type HandlerOutItem struct {
	Identifier broker.Identifier
	Data       interface{}
}

// HandlerOut
type HandlerOut []HandlerOutItem

// HandlerFunc simplified handler func which just accepts message identifier and message data binder
// and provides response consisted of outgoing messages and error which indicates processing status
type HandlerFunc func(identifier broker.Identifier, dataBinder func(dst interface{}) error) (HandlerOut, error)

// WrapHandler wraps handler func doing all stuff
func WrapHandler(handler HandlerFunc) broker.ConsumeFunc {
	return func(broker broker.IBroker, delivery broker.Delivery) error {
		// create data binder
		dataBinder := func(dst interface{}) error {
			err := json.Unmarshal(delivery.Payload(), dst)
			if err != nil {
				rejErr := delivery.Reject()
				if rejErr != nil {
					return errors.MultiErrors{err, rejErr}
				}
				return err
			}
			return nil
		}

		// call handler
		out, err := handler(delivery.Identifier(), dataBinder)
		// in case when error occurs inside handler we must nack message
		if err != nil {
			nackErr := delivery.Nack()
			if nackErr != nil {
				return errors.MultiErrors{err, nackErr}
			}
			return err
		}

		// publish outgoing messages
		var pubErrs []error
		for _, item := range out {
			err := broker.Publish(item.Identifier, item.Data)
			if err != nil {
				pubErrs = append(pubErrs, err)
			}
		}
		if pubErrs != nil {
			return errors.MultiErrors(pubErrs)
		}

		// mark message as successful
		return delivery.Ack()
	}
}
