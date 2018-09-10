package mocks

import (
	"strings"
	"github.com/ericlagergren/decimal"
	"github.com/stretchr/testify/mock"
)

func (c *ICoordinator) GetWalletObserver(coinName string) (obs *IWalletObserver) {
	defer func() {
		r := recover()
		if r != nil {
			if isMockPanic(r) {
				obs = &IWalletObserver{}
				c.On("Observer", coinName).Times(10).Return(obs)
				return
			}
			panic(r)
		}
	}()

	obs = c.Observer(coinName).(*IWalletObserver)
	return
}

func (c *ICoordinator) GetAccountObserver(coinName string) (obs *IAccountObserver) {
	defer func() {
		r := recover()
		if r != nil {
			if isMockPanic(r) {
				obs = &IAccountObserver{}
				c.On("AccountObserver", coinName).Return(obs).Times(10)
				return
			}
			panic(r)
		}
	}()

	obs = c.AccountObserver(coinName).(*IAccountObserver)
	return
}

func (c *ICoordinator) GetTxsSender(coinName string) (ts *ITxSender) {
	defer func() {
		r := recover()
		if r != nil {
			if isMockPanic(r) {
				ts = &ITxSender{}
				c.On("TxsSender", coinName).Return(ts).Times(10)
				return
			}
			panic(r)
		}
	}()

	ts = c.TxsSender(coinName).(*ITxSender)
	return
}


func (wo *IWalletObserver) SetAddressBalance(address string, amount *decimal.Big) {
	wo.On("Balance", mock.Anything, address).Return(amount, nil)
}

func (ao *IAccountObserver) SetAccountBalance(amount *decimal.Big) {
	ao.On("GetBalance", mock.Anything).Return(amount, nil)
}

func (ts *ITxSender) SetSupportInternalTxs(flag bool) {
	ts.On("SupportInternalTxs").Return(flag)
}

func isMockPanic(p interface{}) bool {
	if sr, ok := p.(string); ok {
		if strings.HasPrefix(sr, "\n\nmock: Unexpected Method Call") ||
			strings.HasPrefix(sr, "\nassert: mock:") {
			return true
		}
	}
	return false
}