package providers

import (
	walletconf "git.zam.io/wallet-backend/wallet-api/config/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes/wrappers"
	"git.zam.io/wallet-backend/web-api/pkg/services/sentry"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

// Coordinator
func Coordinator(wConf walletconf.Scheme, logger logrus.FieldLogger, reporter sentry.IReporter) (coordinator nodes.ICoordinator, err error) {
	coordinator = nodes.New(logger)
	for coinName, nodeConf := range wConf.CryptoNodes {
		var additionalParams map[string]interface{}
		switch coinName {
		case "btc", "bch":
			additionalParams = generateBTCNodeAdditionalParams(wConf.BTC)
		case "eth":
			additionalParams, err = generateETHNodeAdditionalParams(wConf.ETH)
			if err != nil {
				return
			}
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
	if wConf.UserReporter {
		logger.Info("applying reporter wrapper onto coordinator")
		if reporter == nil {
			logger.Warn("can't apply reporter wrapper onto coordinator due to it's not provided")
			return
		}
		coordinator = wrappers.NewCoordinatorWrapper(coordinator, reporter)
	}
	return
}

func generateBTCNodeAdditionalParams(conf walletconf.BTCNodeConfiguration) map[string]interface{} {
	return map[string]interface{}{
		"confirmations_count": conf.NeedConfirmationsCount,
	}
}

func generateETHNodeAdditionalParams(conf walletconf.ETHNodeConfiguration) (out map[string]interface{}, err error) {
	err = mapstructure.Decode(&conf, &out)
	return
}