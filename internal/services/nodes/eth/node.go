package eth

import (
	"context"
	"encoding/json"
	"fmt"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes/providers"
	"github.com/danields761/jsonrpc"
	"github.com/ericlagergren/decimal"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultPort      = 8545
	weiOrderOfNumber = 18
)

type netIdT string

var netTypes = map[string]string{
	"1":  "Ethereum Mainnet",
	"2":  "Morden Testnet (deprecated)",
	"3":  "Ropsten Testnet",
	"4":  "Rinkeby Testnet",
	"42": "Kovan Testnet",
}

//
func (n *netIdT) UnmarshalJSON(data []byte) error {
	sData := strings.Replace(string(data), `"`, "", -1)
	val, ok := netTypes[sData]
	if !ok {
		val = fmt.Sprintf("Unknown(%s)", sData)
	}
	*n = netIdT(val)
	return nil
}

func (n *netIdT) IsTestNet() bool {
	return string(*n) != "Ethereum Mainnet"
}

// ethNode
type ethNode struct {
	host              string
	masterPass        string
	logger            logrus.FieldLogger
	rpcClient         jsonrpc.RPCClient
	httpClient        *http.Client
	needConfirmations int

	subscriber func(ctx context.Context, blockHeight int) error

	etherScanParams struct {
		host, apiKey string
	}
}

type configParams struct {
	NeedConfirmationsCount                     int
	MasterPass, EtherscanHost, EtherscanApiKey string
}

// Dial
func Dial(
	logger logrus.FieldLogger,
	addr string, testNet bool,
	additionalParams map[string]interface{},
) (io.Closer, error) {
	logger = logger.WithField("module", "eth.nodes")

	// if port not specified applies default BTC port for selected network type
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%d", addr, defaultPort)
	}

	// wrap host addr specifying http scheme
	if !strings.Contains(addr, "http://") || !strings.Contains(addr, "http://") {
		addr = fmt.Sprintf("http://%s", addr)
	}

	// map additional params
	var params configParams
	err := mapstructure.Decode(additionalParams, &params)
	if err != nil {
		return nil, wrapNodeErr(err)
	}

	// check etherscan params
	_, err = url.Parse(params.EtherscanHost)
	if err != nil {
		return nil, wrapNodeErr(err, "invalid etherscan host param")
	}

	//
	httpClient := &http.Client{}
	node := &ethNode{
		host:              addr,
		masterPass:        params.MasterPass,
		logger:            logger,
		rpcClient:         jsonrpc.NewClientWithOpts(addr, &jsonrpc.RPCClientOpts{HTTPClient: httpClient}),
		httpClient:        httpClient,
		needConfirmations: params.NeedConfirmationsCount,
		etherScanParams:   struct{ host, apiKey string }{host: params.EtherscanHost, apiKey: params.EtherscanApiKey},
	}
	var netId netIdT
	err = node.doRPCCall(context.Background(), "net_version", &netId)
	if err != nil {
		return nil, wrapNodeErr(err)
	}
	if netId.IsTestNet() != testNet {
		if testNet {
			err = fmt.Errorf("testnet is required, but net id is %s", netId)
		} else {
			err = fmt.Errorf("testnet isn't required, but net id is %s", netId)
		}
		return nil, wrapNodeErr(err)
	}

	return node, nil
}

// Close implements io.Closer interface, does nothing since there is no keep-alive connections
func (n *ethNode) Close() error {
	if n.httpClient.Transport != nil {
		t, ok := n.httpClient.Transport.(*http.Transport)
		if !ok {
			return nil
		}
		t.CloseIdleConnections()
	}
	return nil
}

// getMasterPass returns master password used to manipulate with wallets
func (node *ethNode) getMasterPass() string {
	return node.masterPass
}

// test implementation
var _ nodes.IGenerator = (*ethNode)(nil)
var _ nodes.IWalletObserver = (*ethNode)(nil)
var _ nodes.IAccountObserver = (*ethNode)(nil)
var _ nodes.ITxsObserver = (*ethNode)(nil)
var _ nodes.IWatcherLoop = (*ethNode)(nil)
var _ nodes.ITxSender = (*ethNode)(nil)

// Create new account using personal_newAccount rpc method
func (node *ethNode) Create(ctx context.Context) (address string, err error) {
	err = node.doRPCCall(ctx, "personal_newAccount", &address, node.getMasterPass())
	return
}

// Balance
func (node *ethNode) Balance(ctx context.Context, address string) (balance *decimal.Big, err error) {
	var bal hexutil.Big
	err = node.doRPCCall(ctx, "eth_getBalance", &bal, address, "latest")
	if err != nil {
		err = coerceErr(err)
		return
	}
	balance = new(decimal.Big).SetBigMantScale((*big.Int)(&bal), weiOrderOfNumber)
	return
}

// GetBalance implements IAccountObserver by summing balances of all node addresses obtained with eth_accounts rpc-call
func (node *ethNode) GetBalance(ctx context.Context) (balance *decimal.Big, err error) {
	// select balances in separate goroutines, can't use atomic algebra for calculation because of using big int
	type resT struct {
		val big.Int
		err error
	}
	resChan := make(chan resT)

	go node.foreachAddress(
		ctx,
		func(ctx context.Context, address string) {
			var bal hexutil.Big
			qErr := node.doRPCCall(ctx, "eth_getBalance", &bal, address, "latest")
			if qErr != nil {
				qErr = coerceErr(qErr)
			}
			resChan <- resT{
				val: big.Int(bal),
				err: qErr,
			}
		},
		func() {
			close(resChan)
		},
	)

	// calculate total balance
	var (
		totalBalance big.Int
		rErr         error
	)
	for r := range resChan {
		if r.err != nil {
			rErr = merrors.Append(rErr, r.err)
		} else {
			totalBalance.Add(&totalBalance, &r.val)
		}
	}
	if rErr != nil {
		err = rErr
	} else {
		balance = new(decimal.Big).SetBigMantScale(&totalBalance, weiOrderOfNumber)
	}

	return
}

// IsConfirmed
func (node *ethNode) IsConfirmed(ctx context.Context, hash string) (confirmed, abandoned bool, err error) {
	// get best block index to estimate transaction confirmations
	bestBlockIndex, err := node.getBestBlockIndex(ctx)
	if err != nil {
		return
	}

	// get transaction details
	var txDetails struct {
		BlockNumber *hexutil.Uint `json:"blockNumber"`
	}
	err = node.doESCall(ctx, "proxy", "eth_getTransactionByHash", &txDetails, map[string]interface{}{"txhash": hash})
	if err != nil {
		return
	}

	// if block number is't specified, that means that transaction still in txs pool
	if txDetails.BlockNumber == nil {
		confirmed = false
		return
	}

	// compare necessary confirmations count with latest block index - tx block index
	confirmations := bestBlockIndex - int(*txDetails.BlockNumber)
	confirmed = confirmations >= node.needConfirmations
	return
}

// GetIncoming
func (node *ethNode) GetIncoming(ctx context.Context) (txs []nodes.IncomingTxDescr, err error) {
	type resT struct {
		descrs []nodes.IncomingTxDescr
		err    error
	}
	resChan := make(chan resT)

	go node.foreachAddress(
		ctx,
		func(ctx context.Context, address string) {
			var res []struct {
				Confirmations int    `json:"confirmations,string"`
				Value         int64  `json:"value,string"`
				IsErr         int    `json:"isError,string"`
				To            string `json:"to"`
				Hash          string `json:"hash"`
			}
			qErr := node.doESCall(ctx, "account", "txlist", &res, map[string]interface{}{
				"address": address,
			})
			// if rate limit error occurs, repeat it with 1 sec delay
			for qErr == errESRateLimit {
				select {
				case <-ctx.Done():
				case <-time.Tick(time.Second):
					qErr = node.doESCall(ctx, "account", "txlist", &res, map[string]interface{}{
						"address": address,
					})
				}
			}
			if qErr != nil {
				resChan <- resT{err: qErr}
				return
			}
			if len(res) == 0 {
				return
			}

			descrs := make([]nodes.IncomingTxDescr, 0, len(res))
			for _, tDescr := range res {
				if tDescr.To != address {
					continue
				}
				descrs = append(
					descrs,
					nodes.IncomingTxDescr{
						Hash:      tDescr.Hash,
						Address:   address,
						Confirmed: tDescr.Confirmations >= node.needConfirmations,
						Abandoned: tDescr.IsErr == 1,
						Amount:    new(decimal.Big).SetMantScale(tDescr.Value, weiOrderOfNumber),
					},
				)
			}
			resChan <- resT{descrs: descrs}
		},
		func() {
			close(resChan)
		},
	)

	var rErr error
	for r := range resChan {
		if r.err != nil {
			rErr = merrors.Append(rErr, r.err)
		} else {
			txs = append(txs, r.descrs...)
		}
	}
	if rErr != nil {
		err = rErr
	}
	return
}

// Send
func (node *ethNode) Send(ctx context.Context, fromAddress, toAddress string, amount *decimal.Big) (txHash string, fee *decimal.Big, err error) {
	// unlock wallet first
	err = node.doRPCCall(ctx, "personal_unlockAccount", nil, fromAddress, node.getMasterPass())
	if err != nil {
		return
	}

	// convert eth to wei
	amount = convertToWei(amount).RoundToInt()
	var outAmount big.Int
	amount.Int(&outAmount)

	// then send amount
	err = node.doRPCCall(
		ctx,
		"eth_sendTransaction",
		&txHash,
		[]interface{}{
			struct {
				From  string      `json:"from"`
				To    string      `json:"to"`
				Value hexutil.Big `json:"value"`
			}{
				From:  fromAddress,
				To:    toAddress,
				Value: hexutil.Big(outAmount),
			},
		},
	)
	if err != nil {
		return
	}
	var txDetails struct {
		Gas      *hexutil.Big `json:"gas"`
		GasPrice *hexutil.Big `json:"gasPrice"`
	}
	err = node.doRPCCall(ctx, "eth_getTransactionByHash", &txDetails, txHash)
	if err != nil {
		return
	}

	fee = new(decimal.Big).SetBigMantScale(
		new(big.Int).Mul((*big.Int)(txDetails.Gas), (*big.Int)(txDetails.GasPrice)),
		weiOrderOfNumber,
	)

	return
}

func (node *ethNode) getBestBlockIndex(ctx context.Context) (index int, err error) {
	var indexRes hexutil.Uint
	err = node.doRPCCall(ctx, "eth_blockNumber", &indexRes)
	index = int(indexRes)
	return
}

// SupportInternalTxs eth doesn't supports internal txs
func (node *ethNode) SupportInternalTxs() bool {
	return false
}

func (node *ethNode) foreachAddress(
	ctx context.Context,
	f func(ctx context.Context, address string),
	callAfter func(),
) (err error) {
	// request node wallets
	var addresses []string
	err = node.doRPCCall(ctx, "eth_accounts", &addresses)
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(addresses))
	for _, a := range addresses {
		go func(a string) {
			f(ctx, a)
			wg.Done()
		}(a)
	}
	wg.Wait()
	callAfter()

	return nil
}

func (node *ethNode) doRPCCall(ctx context.Context, method string, output interface{}, params ...interface{}) (err error) {
	l := node.logger.WithField("method", method)

	l.WithField("params", params).Debug("calling rpc")

	resp, err := node.rpcClient.Call(method, params...)
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
	l.WithField("resp", resp.Result).Debug("successfully received response")
	return
}

var errESRateLimit = errors.New("eth node: ES rate limit")

func (node *ethNode) doESCall(
	ctx context.Context,
	module, action string,
	output interface{},
	params map[string]interface{},
) error {
	l := node.logger.WithField("es_module", module).WithField("es_action", action)

	l.WithField("params", params).Debug("making REST api call")

	qVals := url.Values{}
	qVals.Set("apikey", node.etherScanParams.apiKey)
	qVals.Set("module", module)
	qVals.Set("action", action)
	for k, v := range params {
		qVals.Set(k, fmt.Sprint(v))
	}

	u, err := url.Parse(node.etherScanParams.host)
	if err != nil {
		return wrapNodeErr(err)
	}
	u.RawQuery = qVals.Encode()

	l.WithField("url", u.String()).Debug("using url")

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return wrapNodeErr(err)
	}

	// do request
	httpResp, err := node.httpClient.Do(req)
	if err != nil {
		return wrapNodeErr(err)
	}

	if httpResp.StatusCode == 403 {
		return errESRateLimit
	}

	if module == "proxy" {
		resp := jsonrpc.RPCResponse{}
		err = json.NewDecoder(httpResp.Body).Decode(&resp)
		if err != nil {
			return wrapNodeErr(err)
		}

		if resp.Error != nil {
			return wrapNodeErr(resp.Error)
		}

		return wrapNodeErr(json.Unmarshal([]byte(resp.Result), output), "escan: decode body")
	} else {
		// unmarshal result
		var resp struct {
			Status  int             `json:"status,string"`
			Message string          `json:"message"`
			Result  json.RawMessage `json:"result"`
		}
		err = json.NewDecoder(httpResp.Body).Decode(&resp)
		if err != nil {
			return wrapNodeErr(err, "escan: decode")
		}
		if resp.Status == 0 {
			// ignore no transactions found error
			if resp.Message == "No transactions found" {
				return nil
			}
			return wrapNodeErr(fmt.Errorf("escan: response with '%s'", resp.Message))
		}

		return wrapNodeErr(json.Unmarshal(resp.Result, output), "escan: decode body")
	}
}

func wrapNodeErr(err error, descr ...string) error {
	if err == nil {
		return nil
	}
	if len(descr) > 0 {
		return errors.Wrapf(err, "eth node: %s", descr[0])
	}
	return errors.Wrap(err, "eth node")
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

func convertToWei(amount *decimal.Big) *decimal.Big {
	return new(decimal.Big).Set(amount).SetScale(amount.Scale() - weiOrderOfNumber)
}

func convertToEth(amount *decimal.Big) *decimal.Big {
	return new(decimal.Big).Set(amount).SetScale(amount.Scale() + weiOrderOfNumber)
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
	providers.Register("eth", provider{})
}
