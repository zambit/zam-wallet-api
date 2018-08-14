package icex

import (
	"context"
	"encoding/json"
	"fmt"
	"git.zam.io/wallet-backend/common/pkg/validations"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"github.com/ericlagergren/decimal"
	"github.com/go-playground/validator"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"strings"
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
}

type response struct {
	Result bool           `json:"result" validate:"required"`
	Data   []responseItem `json:"data" validate:"required,len=1"`
}

// ICoinConverter uses icex.ch service to convert values
type CoinConverter struct {
	Client   http.Client
	ICEXHost string
}

// New coin converter which uses icex.ch service, requires icex host parameter.
func New(icexHost string) helpers.ICoinConverter {
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
	return &CoinConverter{ICEXHost: icexHost}
}

// ConvertToFiat implements ICoinConverter
func (converter *CoinConverter) ConvertToFiat(
	ctx context.Context,
	coinName string,
	amount *decimal.Big,
	dstCurrencyName string,
) (fiatAmount *decimal.Big, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "icex_convert_to_fiat")
	defer span.Finish()

	// check crypto-coin name
	icexCoinName, ok := coinNameToIcexNameMapping[strings.ToLower(coinName)]
	if !ok {
		return nil, helpers.ErrCryptoCurrencyName
	}

	// check fiat-currency supported
	_, ok = fiatNamesSet[strings.ToUpper(dstCurrencyName)]
	if !ok {
		return nil, helpers.ErrFiatCurrencyName
	}

	//
	endpointUrl := converter.ICEXHost + "/api/coins/" + icexCoinName + "?convert=" + dstCurrencyName
	span.LogKV("icex_coin_name", icexCoinName)
	span.LogKV("icex_endpoint_url", endpointUrl )
	req, err := http.NewRequest("GET", endpointUrl , nil)
	if err != nil {
		err = errors.Wrap(err, "icex_convert: icex host unavailable")
		return
	}

	resp, err := converter.Client.Do(req.WithContext(ctx))
	if err != nil {
		err = helpers.ErrCryptoCurrencyName
		return
	}
	// wrong response means service unavailable
	if resp.StatusCode != 200 && resp.Header.Get("Content-type") != "application/json" {
		err = errors.Wrap(helpers.ErrUnavailable, "icex_convert: either status code not 200 or content type is wrong")
		return
	}

	// unmarshal result
	var dst response
	err = json.NewDecoder(resp.Body).Decode(&dst)
	if err != nil {
		err = errors.Wrap(err, "icex_convert: error occurs while decoding response")
		return
	}

	// check response using validate to ensure fields are valid
	err = v.Struct(&dst)
	if err != nil {
		err = errors.Wrap(err, "icex_convert: error while validating remote response")
		return
	}

	// calculate result
	if amount == nil {
		amount = new(decimal.Big)
	}
	fiatAmount = new(decimal.Big).Mul(amount, new(decimal.Big).SetFloat64(dst.Data[0].Price.Value))
	return
}
