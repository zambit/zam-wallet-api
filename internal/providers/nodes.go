package providers

import (
	walletconf "git.zam.io/wallet-backend/wallet-api/config/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"github.com/sirupsen/logrus"
)

// Coordinator
func Coordinator(wConf walletconf.Scheme, logger logrus.FieldLogger) (coordinator nodes.ICoordinator, err error) {
	coordinator = nodes.New(logger)
	for coinName, nodeConf := range wConf.CryptoNodes {
		var additionalParams map[string]interface{}
		switch coinName {
		case "btc", "bch":
			additionalParams = generateBTCNodeAdditionalParams(wConf.BTC)
		}

		logger.WithField(
			"conn_params", nodeConf,
		).WithField(
			"additional_params", additionalParams,
		).Infof("connecting %s node", coinName)

		err = coordinator.Dial(coinName, nodeConf.Host, nodeConf.User, nodeConf.Pass, nodeConf.Testnet, additionalParams)
		if err != nil {
			logger.WithError(err).Errorf("connecting node %s has been failed", coinName)
			return
		}
	}
	return
}

func generateBTCNodeAdditionalParams(conf walletconf.BTCNodeConfiguration) map[string]interface{} {
	return map[string]interface{}{
		"confirmations_count": conf.NeedConfirmationsCount,
	}
}
