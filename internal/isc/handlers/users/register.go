package users

import (
	"git.zam.io/wallet-backend/wallet-api/internal/isc/handlers/base"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/web-api/db"
	"git.zam.io/wallet-backend/web-api/pkg/services/broker"
	"github.com/sirupsen/logrus"
)

// Register
func Register(broker broker.IBroker, d *db.Db, api *wallets.Api, logger logrus.FieldLogger) error {
	return broker.Consume(
		"users", "registration_verification_completed_event",
		base.WrapHandler(RegistrationCompletedFactory(d, api, logger)),
	)
}
