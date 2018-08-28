// Code generated by mockery v1.0.0. DO NOT EDIT.
package mocks

import mock "github.com/stretchr/testify/mock"
import nodes "git.zam.io/wallet-backend/wallet-api/internal/services/nodes"

// ICoordinator is an autogenerated mock type for the ICoordinator type
type ICoordinator struct {
	mock.Mock
}

// AccountObserver provides a mock function with given fields: coinName
func (_m *ICoordinator) AccountObserver(coinName string) nodes.IAccountObserver {
	ret := _m.Called(coinName)

	var r0 nodes.IAccountObserver
	if rf, ok := ret.Get(0).(func(string) nodes.IAccountObserver); ok {
		r0 = rf(coinName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(nodes.IAccountObserver)
		}
	}

	return r0
}

// Close provides a mock function with given fields:
func (_m *ICoordinator) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Dial provides a mock function with given fields: coinName, host, user, pass, testnet
func (_m *ICoordinator) Dial(coinName string, host string, user string, pass string, testnet bool) error {
	ret := _m.Called(coinName, host, user, pass, testnet)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string, string, bool) error); ok {
		r0 = rf(coinName, host, user, pass, testnet)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Generator provides a mock function with given fields: coinName
func (_m *ICoordinator) Generator(coinName string) nodes.IGenerator {
	ret := _m.Called(coinName)

	var r0 nodes.IGenerator
	if rf, ok := ret.Get(0).(func(string) nodes.IGenerator); ok {
		r0 = rf(coinName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(nodes.IGenerator)
		}
	}

	return r0
}

// Observer provides a mock function with given fields: coinName
func (_m *ICoordinator) Observer(coinName string) nodes.IWalletObserver {
	ret := _m.Called(coinName)

	var r0 nodes.IWalletObserver
	if rf, ok := ret.Get(0).(func(string) nodes.IWalletObserver); ok {
		r0 = rf(coinName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(nodes.IWalletObserver)
		}
	}

	return r0
}

// TxsObserver provides a mock function with given fields: coinName
func (_m *ICoordinator) TxsObserver(coinName string) nodes.ITxsObserver {
	ret := _m.Called(coinName)

	var r0 nodes.ITxsObserver
	if rf, ok := ret.Get(0).(func(string) nodes.ITxsObserver); ok {
		r0 = rf(coinName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(nodes.ITxsObserver)
		}
	}

	return r0
}
