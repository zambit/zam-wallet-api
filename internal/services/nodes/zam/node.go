package zam

import (
	"context"
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes/providers"
	"github.com/andskur/go/clients/horizon"
	"github.com/danields761/jsonrpc"
	"github.com/ericlagergren/decimal"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strings"
)

const defaultPort = 443

// zamNode
type zamNode struct {
	host            string
	assetName       string
	issuerPublicKey string
	logger          logrus.FieldLogger
	httpClient      *http.Client
}

type configParams struct {
	AssetName, IssuerPublicKey string
}

// Dial
func Dial(
	logger logrus.FieldLogger,
	addr string, testNet bool,
	additionalParams map[string]interface{},
) (io.Closer, error) {
	logger = logger.WithField("module", "zam.nodes")

	// if port not specified applies default BTC port for selected network typeÅ
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%d", addr, defaultPort)
	}

	// wrap host addr specifying http scheme
	if !strings.Contains(addr, "https://") || !strings.Contains(addr, "https://") {
		addr = fmt.Sprintf("https://%s", addr)
	}

	// map additional params
	var params configParams
	err := mapstructure.Decode(additionalParams, &params)
	if err != nil {
		return nil, wrapNodeErr(err)
	}

	//
	httpClient := &http.Client{}
	node := &zamNode{
		host:            addr,
		assetName:       params.AssetName,
		issuerPublicKey: params.IssuerPublicKey,
		logger:          logger,
		httpClient:      httpClient,
	}
	/*	err = node.doRPCCall(context.Background(), "net_version", &netId)
		if err != nil {
			return nil, wrapNodeErr(err)
		}*/
	/*	if netId.IsTestNet() != testNet {
		if testNet {
			err = fmt.Errorf("testnet is required, but net id is %s", netId)
		} else {
			err = fmt.Errorf("testnet isn't required, but net id is %s", netId)
		}
		return nil, wrapNodeErr(err)
	}*/

	return node, nil
}

// Close implements io.Closer interface, does nothing since there is no keep-alive connections
func (n *zamNode) Close() error {
	if n.httpClient.Transport != nil {
		t, ok := n.httpClient.Transport.(*http.Transport)
		if !ok {
			return nil
		}
		t.CloseIdleConnections()
	}
	return nil
}

// test implementation
var _ nodes.IGenerator = (*zamNode)(nil)
var _ nodes.IWalletObserver = (*zamNode)(nil)

//var _ nodes.IAccountObserver = (*zamNode)(nil)
//var _ nodes.ITxsObserver = (*zamNode)(nil)
//var _ nodes.IWatcherLoop = (*zamNode)(nil)
//var _ nodes.ITxSender = (*zamNode)(nil)

// Create new account using personal_newAccount rpc method
func (node *zamNode) Create(ctx context.Context) (address string, err error) {
	//err = node.doRPCCall(ctx, "personal_newAccount", &address)
	return
}

// Balance
func (node *zamNode) Balance(ctx context.Context, address string) (balance *decimal.Big, err error) {

	//Get data from Stellar blockchain
	account, err := horizon.DefaultTestNetClient.LoadAccount(address)
	if err != nil {
		err = coerceErr(err)
		return
	}

	//convert balance string to decimal int
	balance = new(decimal.Big)
	balance.SetString(account.Balances[0].Balance)
	return
}

// SupportInternalTxs Zam doesn't supports internal txs
func (node *zamNode) SupportInternalTxs() bool {
	return false
}

func wrapNodeErr(err error, descr ...string) error {
	if err == nil {
		return nil
	}
	if len(descr) > 0 {
		return errors.Wrapf(err, "zam node: %s", descr[0])
	}
	return errors.Wrap(err, "zam node")
}

const (
	errCodeInvalidAddress = -32602
)

func coerceErr(err error) error {
	if rErr, ok := err.(*jsonrpc.RPCError); ok {
		switch rErr.Code {
		case errCodeInvalidAddress:
			return nodes.ErrAddressInvalid
		}
	}
	return err
}

// register dialer
type provider struct{}

func (p provider) Dial(
	logger logrus.FieldLogger,
	host, user, pass string,
	testnet bool,
	additionalParams map[string]interface{},
) (io.Closer, error) {
	return Dial(logger, host, testnet, additionalParams)
}

func init() {
	providers.Register("zam", provider{})
}
