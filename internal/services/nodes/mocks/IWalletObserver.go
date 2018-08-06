// Code generated by mockery v1.0.0
package mocks

import decimal "github.com/ericlagergren/decimal"
import mock "github.com/stretchr/testify/mock"

// IWalletObserver is an autogenerated mock type for the IWalletObserver type
type IWalletObserver struct {
	mock.Mock
}

// Balance provides a mock function with given fields: address
func (_m *IWalletObserver) Balance(address string) (*decimal.Big, error) {
	ret := _m.Called(address)

	var r0 *decimal.Big
	if rf, ok := ret.Get(0).(func(string) *decimal.Big); ok {
		r0 = rf(address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*decimal.Big)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
