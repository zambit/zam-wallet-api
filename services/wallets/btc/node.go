package btc

import (
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/services/wallets/providers"
	"github.com/danields761/jsonrpc"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strings"
)

type btcNode struct {
	logger    logrus.FieldLogger
	client    *http.Client
	rpcClient jsonrpc.RPCClient
}

func Dial(addr, user, pass string, testnet bool) (io.Closer, error) {
	httpClient := &http.Client{}
	// set basic auth
	if user != "" && pass != "" {
		httpClient = HttpClientWithBasicAuth(httpClient, user, pass)
	}

	if !strings.Contains(addr, ":") {
		port := 8332
		if testnet {
			port = 18332
		}
		addr = fmt.Sprintf("%s:%d", addr, port)
	}

	if !strings.Contains(addr, "http://") || !strings.Contains(addr, "http://") {
		addr = fmt.Sprintf("http://%s", addr)
	}

	n := &btcNode{
		logger: logrus.StandardLogger().WithField("module", "wallets.btc.rpc"),
		client: httpClient,
		rpcClient: jsonrpc.NewClientWithOpts(
			addr, &jsonrpc.RPCClientOpts{HTTPClient: httpClient},
		),
	}
	err := n.Ping()
	if err != nil {
		return nil, err
	}

	return n, nil
}

// Close implements io.Closer interface, does nothing since there is no keep-alive connections
func (n *btcNode) Close() error {
	return nil
}

// Ping node by calling getwalletinfo
func (n *btcNode) Ping() (err error) {
	resp, err := n.rpcClient.Call("getwalletinfo")
	if err == nil && resp.Error != nil {
		err = resp.Error
	}
	if err != nil {
		n.logger.WithError(err).Error("getwalletinfo call error")
		return
	}
	n.logger.WithField("resp", resp.Result).Info("successfully receive response")
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
type provider struct{}

func (provider) Dial(host, user, pass string, testnet bool) (io.Closer, error) {
	return Dial(host, user, pass, testnet)
}

func init() {
	providers.Register("btc", provider{})
	providers.Register("bch", provider{})
}
