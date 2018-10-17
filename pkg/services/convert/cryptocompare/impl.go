package cryptocompare

import (
	"bytes"
	"context"
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"github.com/ericlagergren/decimal"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/segmentio/objconv/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

//TODO Transfer ZamValue to Config
const (
	multiPricePath = "/data/pricemulti"
	ZamValue       = 0.02
)

type queryParams struct {
	From []string `url:"fsyms"`
	To   string   `url:"tsyms"`
}

type responseBody map[string]map[string]float64

// ICoinConverter uses cryptocompare.com shitty api for getting currencies rates values
type CryptoCurrency struct {
	client *http.Client
	host   string
}

// New coin converter which uses icex.ch service, requires icex host parameter.
func New(serviceHost string) (convert.ICryptoCurrency, error) {
	if serviceHost == "" {
		return nil, fmt.Errorf("cryptocompare converter: service host parameter is required")
	} else {
		fullEndpoint := serviceHost
		_, err := url.Parse(fullEndpoint)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"cryptocompare converter: service host parameter seems to be invalid: err on parse url %s",
				fullEndpoint,
			)
		}
	}
	return &CryptoCurrency{host: serviceHost, client: &http.Client{}}, nil
}

// GetRate implements ICryptoCurrency
func (c *CryptoCurrency) GetRate(ctx context.Context, coinName string, dstCurrencyName string) (rate *convert.Rate, err error) {
	resp, err := c.doQuery(ctx, []string{coinName}, dstCurrencyName)
	/*if err != nil {
		return
	}*/

	coinName = strings.ToUpper(coinName)
	dstCurrencyName = strings.ToUpper(dstCurrencyName)

	if coinVals, ok := resp[coinName]; ok {
		if rateVal, ok := coinVals[dstCurrencyName]; ok {
			rate = new(convert.Rate)
			(*decimal.Big)(rate).SetFloat64(rateVal)
		} else {
			err = convert.ErrFiatCurrencyName
		}
	} else {
		// check if coin is ZAM token
		if coinName == "ZAM" {
			rate = new(convert.Rate)
			(*decimal.Big)(rate).SetFloat64(ZamValue)
		} else {
			err = convert.ErrCryptoCurrencyName
		}
	}
	return
}

// GetMultiRate implements ICryptoCurrency
func (c *CryptoCurrency) GetMultiRate(ctx context.Context, coinNames []string, dstCurrencyName string) (mr convert.MultiRate, err error) {
	resp, err := c.doQuery(ctx, coinNames, dstCurrencyName)
	if err != nil {
		return
	}

	mr = make(convert.MultiRate, len(coinNames))
	// lookup values
	for _, coinName := range coinNames {
		coinNameUp := strings.ToUpper(coinName)
		dstCurrencyName = strings.ToUpper(dstCurrencyName)

		if coinVals, ok := resp[coinNameUp]; ok {
			if rateVal, ok := coinVals[dstCurrencyName]; ok {
				val := decimal.Big{}
				val.SetFloat64(rateVal)
				mr[coinName] = convert.Rate(val)
			}
			// ignore missed currencies
		} else {
			// check if coin is ZAM token
			if coinNameUp == "ZAM" {
				val := decimal.Big{}
				val.SetFloat64(ZamValue)
				mr[coinName] = convert.Rate(val)
			} else {
				err = convert.ErrCryptoCurrencyName
			}
		}
	}
	return
}

func (c *CryptoCurrency) doQuery(ctx context.Context, coinNames []string, dstCurrencyName string) (resp responseBody, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "cyptocompare_do_query")
	defer span.Finish()

	// uppercase all currencies
	for i, name := range coinNames {
		coinNames[i] = strings.ToUpper(name)
	}
	if dstCurrencyName == "" {
		err = errors.New("cryptocompare converter: empty dst currency value")
		return
	}
	dstCurrencyName = strings.ToUpper(dstCurrencyName)

	u, _ := url.Parse(c.host)
	u.Path = multiPricePath
	v := url.Values{}
	v.Set("fsyms", strings.Join(coinNames, ","))
	v.Set("tsyms", dstCurrencyName)
	u.RawQuery = v.Encode()
	req, _ := http.NewRequest("GET", u.String(), nil)

	span.LogKV("convert_url", req.URL.String())

	// perform query
	r, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		return
	}

	// TODO decode from request stream when debug will be offed
	data, rErr := ioutil.ReadAll(r.Body)
	span.LogKV("resp_code", r.StatusCode)
	span.LogKV("resp_body", string(data))
	if r.StatusCode != 200 {
		if rErr != nil {
			err = errors.Wrap(rErr, "cryptocompare converter: another error occurs while reading error response")
			return
		}
		err = fmt.Errorf("cryptocompare converter: service response: %s", string(data))
		return
	}

	// unmarshal response
	err = json.NewDecoder(bytes.NewReader(data)).Decode(&resp)
	return
}
