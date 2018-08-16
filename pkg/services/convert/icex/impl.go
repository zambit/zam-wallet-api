package icex

import (
	"context"
	"encoding/json"
	"fmt"
	"git.zam.io/wallet-backend/common/pkg/validations"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"github.com/ericlagergren/decimal"
	"github.com/go-playground/validator"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"strings"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
)

// static coin name mapping
var coinNameToIcexNameMapping = map[string]string{
	"btc": "bitcoin",
	"bch": "bitcoin-cash",
	"eth": "ethereum",
}

// fiatNamesSet fixed set of supported currencies
var fiatNamesSet = map[string]struct{}{
	"AUD": {},
	"BRL": {},
	"CAD": {},
	"CHF": {},
	"CNY": {},
	"CZK": {},
	"DKK": {},
	"EUR": {},
	"GBP": {},
	"HKD": {},
	"HUF": {},
	"IDR": {},
	"ILS": {},
	"INR": {},
	"JPY": {},
	"KRW": {},
	"MXN": {},
	"MYR": {},
	"NOK": {},
	"NZD": {},
	"PHP": {},
	"PLN": {},
	"RUB": {},
	"SEK": {},
	"SGD": {},
	"THB": {},
	"TRY": {},
	"ZAR": {},
	"USD": {},
}

const (
	coinsEndpoint     = "/api/coins"
	coinValueEndpoint = "/api/coins/"
)

// v is validator
var v = validator.New()

func init() {
	v.RegisterTagNameFunc(validations.JsonTagNameFunc)
	v.RegisterValidation("phone", validations.PhoneValidator)
}

type responseItem struct {
	Price struct {
		Value float64 `json:"value" validate:"required"`
	} `json:"price" validate:"required"`

	Short string `json:"short"`
}

type response struct {
	Result bool           `json:"result" validate:"required"`
	Data   []responseItem `json:"data" validate:"required,min=1"`
}

// ICoinConverter uses icex.ch service get currencies rates values
type CryptoCurrency struct {
	Client   http.Client
	ICEXHost string
}

// New coin converter which uses icex.ch service, requires icex host parameter.
func New(icexHost string) convert.ICryptoCurrency {
	if icexHost == "" {
		panic(fmt.Errorf("icex_converter: icex host parameter is required"))
	} else {
		fullEndpoint := icexHost + coinsEndpoint
		_, err := url.Parse(fullEndpoint)
		if err != nil {
			panic(errors.Wrapf(
				err,
				"icex_converter: icex host parameter seems to be invalid: err on parse url 'host+/api/coins' (%s)",
				fullEndpoint,
			))
		}
	}
	return &CryptoCurrency{ICEXHost: icexHost}
}

// ToFiat implements ICryptoCurrency, uses opentracing to trace requests
func (converter *CryptoCurrency) GetRate(
	ctx context.Context,
	coinName string,
	dstCurrencyName string,
) (rate *convert.Rate, err error) {
	err = trace.InsideSpanE(ctx, "icex_get_rate", func(ctx context.Context, span opentracing.Span) error {
		dst, err := converter.getRates(ctx, []string{coinName}, dstCurrencyName)
		if err != nil {
			return err
		}

		// create rate converter
		rate = (*convert.Rate)(new(decimal.Big).SetFloat64(dst.Data[0].Price.Value))

		return nil
	})
	if err != nil {
		return
	}
	return
}

// ToFiat implements ICryptoCurrency, uses opentracing to trace requests
func (converter *CryptoCurrency) GetMultiRate(
	ctx context.Context,
	coinNames[]string,
	dstCurrencyName string,
) (mr convert.MultiRate, err error) {
	if len(coinNames) == 0 {
		err = errors.New("icex_converter: at least two coins in the list required")
		return
	}

	err = trace.InsideSpanE(ctx, "icex_get_multi_rate", func(ctx context.Context, span opentracing.Span) error {
		dst, err := converter.getRates(ctx, coinNames, dstCurrencyName)
		if err != nil {
			return err
		}

		// create rate converter for all currencies
		mr = make(convert.MultiRate, len(dst.Data))
		for _, d := range dst.Data {
			price := decimal.Big{}
			price.SetFloat64(dst.Data[0].Price.Value)
			mr[strings.ToUpper(d.Short)] = (convert.Rate)(price)
		}

		return nil
	})
	if err != nil {
		return
	}
	return
}

func (converter *CryptoCurrency) getRates(ctx context.Context, coins []string, dstCurrencyName string) (dst response, err error) {
	span := opentracing.SpanFromContext(ctx)

	// check crypto-coins names
	for i, coinName := range coins {
		var ok bool
		coins[i], ok = coinNameToIcexNameMapping[strings.ToLower(coinName)]
		if !ok {
			err = convert.ErrCryptoCurrencyName
			return
		}
	}

	// check fiat-currency supported
	_, ok := fiatNamesSet[strings.ToUpper(dstCurrencyName)]
	if !ok {
		err = convert.ErrFiatCurrencyName
		return
	}

	var (
		additionalCurrencies, icexCoinName string
	)
	switch len(coins) {
	case 0:
		err = errors.New("icex_convert: empty coins list")
	case 1:
		icexCoinName = coins[0]
	default:
		icexCoinName = coins[0]
		additionalCurrencies = strings.Join(coins[1:], ",")
	}

	//
	url, err := url.Parse(converter.ICEXHost)
	if err != nil {
		err = errors.Wrap(err, "icex_conert: invalid icex host")
		return
	}
	url.Path = coinValueEndpoint + icexCoinName
	if additionalCurrencies != "" {
		url.RawQuery = "with="+additionalCurrencies
	}

	endpointUrl := url.String()
	span.LogKV("icex_endpoint_url", endpointUrl)
	req, err := http.NewRequest("GET", endpointUrl, nil)
	if err != nil {
		err = errors.Wrap(err, "icex_convert: icex host unavailable")
		return
	}

	resp, err := converter.Client.Do(req.WithContext(ctx))
	if err != nil {
		err = convert.ErrCryptoCurrencyName
		return
	}
	// wrong response means service unavailable
	if resp.StatusCode != 200 || resp.Header.Get("Content-type") != "application/json" {
		err = errors.Wrap(convert.ErrUnavailable, "icex_convert: either status code not 200 or content type is wrong")
		return
	}

	// unmarshal result
	err = json.NewDecoder(resp.Body).Decode(&dst)
	if err != nil {
		err = errors.Wrap(err, "icex_convert: error occurs while decoding response")
		return
	}

	// check response using validate to ensure fields are valid
	err = v.Struct(&dst)
	if err != nil {
		err = errors.Wrap(err, "icex_convert: error while validating remote response")
	}
	return
}