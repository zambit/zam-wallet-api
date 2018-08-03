package btc

import (
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes/providers"
	"github.com/danields761/jsonrpc"
	"github.com/ericlagergren/decimal"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultMainNetPort = 8332
	defaultTestNetPort = 18332

	rpcErrInvalidAddressCode = -5
)

// btcNode implements IGenerator interface for BTC/BCH nodes
type btcNode struct {
	logger    logrus.FieldLogger
	client    *http.Client
	rpcClient jsonrpc.RPCClient
}

// interfaces compile-time validations
var _ nodes.IGenerator = (*btcNode)(nil)
var _ nodes.IWalletObserver = (*btcNode)(nil)

// Dial creates client HTTP connection using passed params, also checks connectivity by sending "getwalletinfo" request.
//
// If port not specified, automatically applies appropriate default port for selected network.
//
// If scheme not specified, automatically applies default http scheme, https must be specified explicitly.
func Dial(logger logrus.FieldLogger, coin, addr, user, pass string, testnet bool) (io.Closer, error) {
	// create client and sets default timeout everywhere
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        5,
			TLSHandshakeTimeout: 5 * time.Second,
		},
		Timeout: time.Second * 10,
	}
	// set basic auth
	if user != "" && pass != "" {
		httpClient = HttpClientWithBasicAuth(httpClient, user, pass)
	}

	// if port not specified applies default BTC port for selected network type
	if !strings.Contains(addr, ":") {
		port := defaultMainNetPort
		if testnet {
			port = defaultTestNetPort
		}
		addr = fmt.Sprintf("%s:%d", addr, port)
	}

	// wrap host addr specifying http scheme
	if !strings.Contains(addr, "http://") || !strings.Contains(addr, "http://") {
		addr = fmt.Sprintf("http://%s", addr)
	}

	n := &btcNode{
		logger: logger.WithField("module", "wallets.btc."+coin),
		client: httpClient,
		rpcClient: jsonrpc.NewClientWithOpts(
			addr, &jsonrpc.RPCClientOpts{HTTPClient: httpClient},
		),
	}
	// ping node
	err := n.Ping()
	if err != nil {
		return nil, err
	}

	return n, nil
}

// Close implements io.Closer interface, does nothing since there is no keep-alive connections
func (n *btcNode) Close() error {
	if n.client.Transport != nil {
		t, ok := n.client.Transport.(*http.Transport)
		if !ok {
			return nil
		}
		t.CloseIdleConnections()
	}
	return nil
}

// Create new BTC wallet chained with root wallet
func (n *btcNode) Create() (address string, err error) {
	err = n.doCall("getnewaddress", &address)
	if err != nil {
		return
	}

	// trim prefixes when wallet appears as "prefix:address"
	if index := strings.IndexRune(address, ':'); index != -1 {
		address = address[index+1:]
	}

	return
}

// bigIntJSONView represent decimal.Big json representation
type bigIntJSONView decimal.Big

func (u *bigIntJSONView) UnmarshalJSON(data []byte) error {
	var (
		ok  bool
		dst *decimal.Big
	)
	dst, ok = decimal.ContextUnlimited.SetString((*decimal.Big)(u), string(data))
	if !ok {
		return fmt.Errorf(`decimal: unable to parse string "%s"`, data)
	}
	*u = *(*bigIntJSONView)(dst)
	return nil
}

// Balance returns wallet balance associated with given address using getreceivedbyaddress method
func (n *btcNode) Balance(address string) (balance *decimal.Big, err error) {
	var inputBalance bigIntJSONView
	err = n.doCall("getreceivedbyaddress", &inputBalance, address)
	if rpcErr, ok := err.(*jsonrpc.RPCError); ok {
		if rpcErr.Code == rpcErrInvalidAddressCode {
			err = nodes.ErrAddressInvalid
		}
	}
	casted := decimal.Big(inputBalance)
	balance = &casted
	return
}

// Ping node by calling getwalletinfo
func (n *btcNode) Ping() error {
	return n.doCall("getwalletinfo", nil)
}

//
func (n *btcNode) doCall(method string, output interface{}, params ...interface{}) (err error) {
	l := n.logger.WithField("method", method)

	l.WithField("params", params).WithField("method", method).Info("calling rpc")

	resp, err := n.rpcClient.Call(method, params...)
	if err == nil && resp.Error != nil {
		err = resp.Error
	}
	if err != nil {
		l.WithError(err).Error("error occurs")
		return
	}
	if output != nil {
		err = resp.GetObject(output)
		if err != nil {
			l.WithError(err).WithField("resp_body", resp.Result).Error("error occurs while unmarshal response body")
			return
		}
	}
	l.WithField("resp", resp.Result).Info("successfully received response")
	return
}

// HookRequestTransport hooks request object before call
type HookRequestTransport struct {
	transport http.RoundTripper
	hook      func(r *http.Request) *http.Request
}

func (t *HookRequestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.transport.RoundTrip(t.hook(r))
}

func NewRequestHookTransport(transport http.RoundTripper, hook func(*http.Request) *http.Request) http.RoundTripper {
	return &HookRequestTransport{transport, hook}
}

func HttpClientWithBasicAuth(c *http.Client, user, pass string) *http.Client {
	newC := *c
	transport := c.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	newC.Transport = NewRequestHookTransport(transport, func(r *http.Request) *http.Request {
		r.SetBasicAuth(user, pass)
		return r
	})

	return &newC
}

// register dialer
type provider struct {
	coin string
}

func (p provider) Dial(logger logrus.FieldLogger, host, user, pass string, testnet bool) (io.Closer, error) {
	return Dial(logger, p.coin, host, user, pass, testnet)
}

func init() {
	providers.Register("btc", provider{coin: "btc"})
	providers.Register("bch", provider{coin: "bch"})
}
