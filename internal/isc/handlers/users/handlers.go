package users

import (
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/isc/handlers/base"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/web-api/db"
	"git.zam.io/wallet-backend/web-api/pkg/services/broker"
	"github.com/sirupsen/logrus"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"github.com/pkg/errors"
	"context"
)

const componentName = "isc.users.handlers"

// RegistrationCompletedFactory handles user registration event and creates wallets for all available coins
func RegistrationCompletedFactory(d *db.Db, api *wallets.Api, logger logrus.FieldLogger) base.HandlerFunc {
	return func(identifier broker.Identifier, dataBinder func(dst interface{}) error) (out base.HandlerOut, err error) {
		span := opentracing.GlobalTracer().StartSpan("registration_completed_handler")
		ctx := opentracing.ContextWithSpan(context.Background(), span)
		defer span.Finish()

		ext.SpanKind.Set(span, ext.SpanKindConsumerEnum)
		ext.Component.Set(span, componentName)

		// bind params
		params := CreatedEvent{}
		err = dataBinder(&params)
		if err != nil {
			trace.LogErrorWithMsg(span, err, "message parsing failed")
			return
		}
		if params.UserPhone == "" {
			trace.LogError(span, errors.New("user phone is empty"))
			return
		}

		span.LogKV("user_phone", params.UserPhone)

		// query available coins and create set
		coins, err := queries.GetDefaultCoins(d)
		if err != nil {
			trace.LogErrorWithMsg(span, err, "default coins fetch failed")
			return
		}
		coinsNamesSet := make(map[string]struct{})
		for _, c := range coins {
			coinsNamesSet[c.ShortName] = struct{}{}
		}

		// query already created wallets
		wts, _, _, err := api.GetWallets(ctx, params.UserPhone, "", 0, 0)
		if err != nil {
			trace.LogErrorWithMsg(span, err, "user wallets query failed")
			return
		}

		// exclude already created wallets from coins set
		for _, w := range wts {
			if _, ok := coinsNamesSet[w.Coin.ShortName]; ok {
				delete(coinsNamesSet, w.Coin.ShortName)
			}
		}

		// create wallets for all enabled coins
		for _, c := range coins {
			// force default wallet name
			_, cErr := api.CreateWallet(ctx, params.UserPhone, c.ShortName, "")
			if cErr != nil {
				span.LogKV("coin_name", c.ShortName)
				trace.LogErrorWithMsg(span, cErr, "wallet creation failed")
				err = merrors.Append(err, cErr)
			}
		}

		return
	}
}
