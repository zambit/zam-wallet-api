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
		logger.WithField("conn_params", nodeConf).Infof("connecting %s node", coinName)
		err = coordinator.Dial(coinName, nodeConf.Host, nodeConf.User, nodeConf.Pass, nodeConf.Testnet)
		if err != nil {
			logger.WithError(err).Errorf("connecting node %s has been failed", coinName)
			return
		}
	}
	return
}
