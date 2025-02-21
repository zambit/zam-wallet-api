// Code generated by mockery v1.0.0. DO NOT EDIT.
package mocks

import context "context"
import decimal "github.com/ericlagergren/decimal"

import mock "github.com/stretchr/testify/mock"
import queries "git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"

// IBalance is an autogenerated mock type for the IBalance type
type IBalance struct {
	mock.Mock
}

// AccountBalanceCtx provides a mock function with given fields: ctx, coinName
func (_m *IBalance) AccountBalanceCtx(ctx context.Context, coinName string) (*decimal.Big, error) {
	ret := _m.Called(ctx, coinName)

	var r0 *decimal.Big
	if rf, ok := ret.Get(0).(func(context.Context, string) *decimal.Big); ok {
		r0 = rf(ctx, coinName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*decimal.Big)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, coinName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// TotalWalletBalanceCtx provides a mock function with given fields: ctx, wallet
func (_m *IBalance) TotalWalletBalanceCtx(ctx context.Context, wallet *queries.Wallet) (*decimal.Big, error) {
	ret := _m.Called(ctx, wallet)

	var r0 *decimal.Big
	if rf, ok := ret.Get(0).(func(context.Context, *queries.Wallet) *decimal.Big); ok {
		r0 = rf(ctx, wallet)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*decimal.Big)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *queries.Wallet) error); ok {
		r1 = rf(ctx, wallet)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
