package icex

import (
	"context"
	"encoding/json"
	"fmt"
	"git.zam.io/wallet-backend/common/pkg/validations"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"github.com/go-playground/validator"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"strings"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"github.com/ericlagergren/decimal"
)

// static coin name mapping
var coinNameToIcexNameMapping = map[string]string{
	"btc": "bitcoin",
	"bch": "bitcoin-cash",
	"eth": "ethereum",
}

// fiatNamesSet fixed set of supported currencies
var fiatNamesSet = map[string]struct{}{
	"aud": {},
	"brl": {},
	"cad": {},
	"chf": {},
	"cny": {},
	"czk": {},
	"dkk": {},
	"eur": {},
	"gbp": {},
	"hkd": {},
	"huf": {},
	"idr": {},
	"ils": {},
	"inr": {},
	"jpy": {},
	"krw": {},
	"mxn": {},
	"myr": {},
	"nok": {},
	"nzd": {},
	"php": {},
	"pln": {},
	"rub": {},
	"sek": {},
	"sgd": {},
	"thb": {},
	"try": {},
	"zar": {},
	"usd": {},
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
	client   http.Client
	icexHost string
}

// New coin converter which uses icex.ch service, requires icex host parameter.
func New(icexHost string) (convert.ICryptoCurrency, error) {
	if icexHost == "" {
		return nil, fmt.Errorf("icex_converter: icex host parameter is required")
	} else {
		fullEndpoint := icexHost + coinsEndpoint
		_, err := url.Parse(fullEndpoint)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"icex_converter: icex host parameter seems to be invalid: err on parse url 'host+/api/coins' (%s)",
				fullEndpoint,
			)
		}
	}
	return &CryptoCurrency{icexHost: icexHost}, nil
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
		err = errors.New("icex_converter: at least one coin in the list required")
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
			price.SetFloat64(d.Price.Value)
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
	dstCurrencyName = strings.ToLower(dstCurrencyName)
	_, ok := fiatNamesSet[dstCurrencyName]
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

	// parse icex host, value is trusted because verified in constructor
	u, _ := url.Parse(converter.icexHost)
	u.Path = coinValueEndpoint + icexCoinName

	// prepare query
	queryValues := make(url.Values, 2)
	queryValues.Set("convert", dstCurrencyName)
	if additionalCurrencies != "" {
		queryValues.Set("with", additionalCurrencies)
	}
	u.RawQuery = queryValues.Encode()

	endpointUrl := u.String()
	span.LogKV("icex_endpoint_url", endpointUrl)

	req, _ := http.NewRequest("GET", endpointUrl, nil)
	resp, err := converter.client.Do(req.WithContext(ctx))
	if err != nil {
		err = errors.Wrap(err, "icex_convert: icex host unavailable")
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