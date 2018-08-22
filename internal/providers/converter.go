package providers

import (
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert/cryptocompare"
	"git.zam.io/wallet-backend/wallet-api/config/server"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert/icex"
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert/fallback"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CryptoCurrency create configuration defined crypto-coins converter
func CoinConverter(cfg server.Scheme, log logrus.FieldLogger) (c convert.ICryptoCurrency, err error) {
	conv := cfg.Convert

	log = log.WithField("module", "providers.converter")

	log.WithField("type", conv.Type).Info("creating main converter")
	c, err = converterForType(conv.Type, log)
	if err != nil {
		log.WithField("type", conv.Type).WithError(err).Error("error occurs while creating main converter")
		return
	}
	if conv.FallbackType != "" {
		log := log.WithField("fb_type", conv.FallbackType).WithField("fb_timeout", conv.FallbackTimeout)
		log.Info("creating fallback converter")

		if conv.FallbackTimeout == 0 {
			err = errors.New("coin converter provider: fallback converter type specified, but timeout not")
			log.WithError(err).Error("creating fallback converter failed due to wrong params")
			return
		}

		fb, err := converterForType(conv.FallbackType, log)
		if err == nil {
			c = fallback.New(c, fb, conv.FallbackTimeout)
		} else {
			log.WithError(err).Warn("creating fallback converter failed, so it will be used without fallback")
		}
	}
	log.Info("success")
	return c, nil
}

func converterForType(t string, log logrus.FieldLogger) (convert.ICryptoCurrency, error) {
	switch t {
	case "":
		fallthrough
	case "icex":
		log.Info("creating icex converter")
		return icex.New("https://api.icex.ch")
	case "cryptocompare":
		log.Info("creating cryptocompare converter")
		return cryptocompare.New("https://min-api.cryptocompare.com")
	default:
		return nil, fmt.Errorf("coin converter provider: uexpected converter type %s", t)
	}
}
