package fallback

import (
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"context"
	"time"
	"net"
	"github.com/opentracing/opentracing-go"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"github.com/pkg/errors"
)

// CryproCurrency implements converter which uses first converter, if timeout occurs it tries fallback converter
type CryproCurrency struct {
	main     convert.ICryptoCurrency
	fallback convert.ICryptoCurrency
	timeout  time.Duration
}

// New creates new converter with callback and timeout
func New(main, fallback convert.ICryptoCurrency, timeout  time.Duration) convert.ICryptoCurrency {
	return &CryproCurrency{main, fallback, timeout}
}

// GetRate implements ICryptoCurrency. If context already have deadline or done channel, it will not refresh deadline.
func (fc *CryproCurrency) GetRate(ctx context.Context, coinName string, dstCurrencyName string) (rate *convert.Rate, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "fallback_get_rate")
	defer span.Finish()

	err = fc.getFallbackRate(ctx, span, func(ctx context.Context, converter convert.ICryptoCurrency) error {
		rate, err = converter.GetRate(ctx, coinName, dstCurrencyName)
		return err
	})
	return
}

// GetGetMultiRateRate implements ICryptoCurrency. If context already have deadline or done channel, it will not refresh deadline.
func (fc *CryproCurrency) GetMultiRate(ctx context.Context, coinNames []string, dstCurrencyName string) (mr convert.MultiRate, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "fallback_get_multi_rate")
	defer span.Finish()

	err = fc.getFallbackRate(ctx, span, func(ctx context.Context, converter convert.ICryptoCurrency) error {
		// copy coins array because it may be modified inside converter
		nCoinNames := make([]string, len(coinNames))
		copy(nCoinNames, coinNames)
		mr, err = converter.GetMultiRate(ctx, nCoinNames, dstCurrencyName)
		return err
	})
	return
}

func (fc *CryproCurrency) getFallbackRate(
	ctx context.Context,
	span opentracing.Span,
	f func(ctx context.Context, converter convert.ICryptoCurrency) error,
) (err error) {
	trace.InsideSpanE(ctx, "main_converter", func(ctx context.Context, span opentracing.Span) error {
		ctx1 := refreshTimeout(ctx, fc.timeout)
		err = f(ctx1, fc.main)
		return err
	})
	if err != nil {
		// any net error will cause fallback to be used
		if _, ok := errors.Cause(err).(net.Error); !ok {
			return
		}
	} else {
		// return after first attempt if no error occurs
		return
	}

	// use fallback here
	trace.InsideSpanE(ctx, "fallback_converter", func(ctx context.Context, span opentracing.Span) error {
		ctx2 := refreshTimeout(ctx, fc.timeout)
		err = f(ctx2, fc.fallback)
		return err
	})
	if err != nil {
		trace.LogError(span, err)
	}
	return
}

func refreshTimeout(ctx context.Context, timeout time.Duration) context.Context {
	_, dedlineOk := ctx.Deadline()
	if dedlineOk {
		return ctx
	}
	if ctx.Done() != nil {
		return ctx
	}
	ctx, _ = context.WithTimeout(ctx, timeout)
	return ctx
}

