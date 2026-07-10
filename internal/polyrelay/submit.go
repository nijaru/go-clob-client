package polyrelay

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// Relayer orchestration constants, mirroring py-sdk defaults.
const (
	// SubmitRetryAttempts is the number of retries after the initial submit
	// (11 total attempts), matching py's GASLESS_SUBMIT_RETRY_ATTEMPTS.
	SubmitRetryAttempts = 10
	// DepositWalletDeadlineS is the deposit-wallet batch deadline offset (now+N).
	DepositWalletDeadlineS = 600
	// ProxyDefaultGasLimit is the fallback when gas estimation fails or is unset.
	ProxyDefaultGasLimit = 200000
	// MetadataMaxLength caps the user-supplied submission metadata.
	MetadataMaxLength = 500

	// DefaultPollMaxAttempts and DefaultPollInterval mirror py's relayer_max_polls (100)
	// and relayer_poll_frequency_ms (2000).
	DefaultPollMaxAttempts = 100
	DefaultPollInterval    = 2 * time.Second

	safeOperationCall         = 0
	safeOperationDelegatecall = 1
)

// GasEstimator estimates gas for a proxy submission. Returning an error makes
// the submit fall back to ProxyDefaultGasLimit, matching py's try/except around
// eth_estimateGas. A nil estimator also selects the default.
type GasEstimator func(ctx context.Context, from, to common.Address, data []byte) (uint64, error)

// GaslessConfig carries the per-account context the orchestrator signs against.
// The clob package builds this from its Config (chain ID, contract addresses,
// wallet type, signer + wallet addresses).
type GaslessConfig struct {
	WalletType           RelayerTransactionType
	Signer               common.Address // EOA authorizing the relay
	Wallet               common.Address // proxy / Safe / deposit wallet address
	ChainID              *big.Int
	ProxyFactory         common.Address
	DepositWalletFactory common.Address
	RelayHub             common.Address
	SafeMultisend        common.Address
	GasEstimator         GasEstimator // optional

	// PollMaxAttempts and PollInterval override the Wait() poll defaults.
	PollMaxAttempts int
	PollInterval    time.Duration
}

func (c GaslessConfig) pollDefaults() (int, time.Duration) {
	maxPolls := c.PollMaxAttempts
	if maxPolls <= 0 {
		maxPolls = DefaultPollMaxAttempts
	}
	d := c.PollInterval
	if d <= 0 {
		d = DefaultPollInterval
	}
	return maxPolls, d
}

// Handle is a submitted-but-not-necessarily-confirmed gasless transaction. Call
// Wait to poll the relayer until it reaches a terminal state.
type Handle struct {
	transport       *Transport
	TransactionID   string
	TransactionHash string // hash from the /submit response, used as poll fallback
	maxPolls        int
	pollDelay       time.Duration
}

// Wait polls the relayer until the transaction reaches a terminal state.
func (h *Handle) Wait(ctx context.Context) (*TransactionOutcome, error) {
	return PollUntilTerminal(
		ctx,
		h.transport,
		h.TransactionID,
		h.TransactionHash,
		h.maxPolls,
		h.pollDelay,
	)
}

// PrepareGasless signs and submits a batch of calls through the relayer for the
// configured wallet type, retrying transient submit failures. It returns a Handle
// the caller polls (or Waits) for confirmation.
func PrepareGasless(
	ctx context.Context,
	t *Transport,
	cfg GaslessConfig,
	key *ecdsa.PrivateKey,
	calls []TransactionCall,
	metadata string,
) (*Handle, error) {
	if key == nil {
		return nil, ErrNilKey
	}
	if len(calls) == 0 {
		return nil, ErrEmptyCalls
	}
	if len(metadata) > MetadataMaxLength {
		return nil, fmt.Errorf("%w: %d > %d", ErrMetadataTooLong, len(metadata), MetadataMaxLength)
	}

	maxPolls, pollDelay := cfg.pollDefaults()
	for attempt := 0; attempt <= SubmitRetryAttempts; attempt++ {
		resp, err := submitForWalletType(ctx, t, cfg, key, calls, metadata)
		if err == nil {
			return &Handle{
				transport:       t,
				TransactionID:   resp.TransactionID,
				TransactionHash: resp.TransactionHash,
				maxPolls:        maxPolls,
				pollDelay:       pollDelay,
			}, nil
		}
		if attempt == SubmitRetryAttempts || !isRetryableSubmitError(err) {
			return nil, err
		}
		if err := sleepCtx(ctx, backoffWithJitter(pollDelay)); err != nil {
			return nil, err
		}
	}
	// Unreachable: the loop returns on success, last attempt, or non-retryable.
	return nil, ErrUnknownType
}

// DeployDepositWallet submits an unsigned WALLET-CREATE to deploy a new deposit
// wallet for the signer via the relayer. Transient submit failures (429, wallet
// contention) are retried like PrepareGasless; the deploy is unsigned and
// idempotent on the relayer (deterministic by signer+factory), so there is no
// nonce or signature to rebuild between attempts.
func DeployDepositWallet(
	ctx context.Context,
	t *Transport,
	signer, factory common.Address,
	metadata string,
) (*Handle, error) {
	if len(metadata) > MetadataMaxLength {
		return nil, fmt.Errorf("%w: %d > %d", ErrMetadataTooLong, len(metadata), MetadataMaxLength)
	}
	payload := BuildWalletCreate(
		WalletCreateInput{Signer: signer, Factory: factory, Metadata: metadata},
	)
	var resp ExecuteResponse
	for attempt := 0; ; attempt++ {
		var err error
		resp, err = t.Submit(ctx, payload)
		if err == nil {
			break
		}
		if attempt == SubmitRetryAttempts || !isRetryableSubmitError(err) {
			return nil, err
		}
		if err := sleepCtx(ctx, backoffWithJitter(DefaultPollInterval)); err != nil {
			return nil, err
		}
	}
	return &Handle{
		transport:       t,
		TransactionID:   resp.TransactionID,
		TransactionHash: resp.TransactionHash,
		maxPolls:        DefaultPollMaxAttempts,
		pollDelay:       DefaultPollInterval,
	}, nil
}

// submitForWalletType dispatches to the per-scheme submit, which fetches the
// nonce, signs, assembles the payload, and posts it.
func submitForWalletType(
	ctx context.Context,
	t *Transport,
	cfg GaslessConfig,
	key *ecdsa.PrivateKey,
	calls []TransactionCall,
	metadata string,
) (ExecuteResponse, error) {
	switch cfg.WalletType {
	case TransactionTypeWallet:
		return submitDepositWallet(ctx, t, cfg, key, calls, metadata)
	case TransactionTypeProxy:
		return submitProxy(ctx, t, cfg, key, calls, metadata)
	case TransactionTypeSafe:
		return submitSafe(ctx, t, cfg, key, calls, metadata)
	default:
		return ExecuteResponse{}, fmt.Errorf("%w: %s", ErrUnknownType, cfg.WalletType)
	}
}

func submitDepositWallet(
	ctx context.Context,
	t *Transport,
	cfg GaslessConfig,
	key *ecdsa.PrivateKey,
	calls []TransactionCall,
	metadata string,
) (ExecuteResponse, error) {
	params, err := t.FetchExecuteParams(ctx, addrHex(cfg.Signer), TransactionTypeWallet)
	if err != nil {
		return ExecuteResponse{}, err
	}
	deadline := big.NewInt(time.Now().Unix() + DepositWalletDeadlineS)
	sig, err := Sign(TransactionTypeWallet, key, RelayRequest{
		Wallet:   cfg.Wallet,
		Calls:    calls,
		Nonce:    params.Nonce,
		Deadline: deadline,
		ChainID:  cfg.ChainID,
	})
	if err != nil {
		return ExecuteResponse{}, err
	}
	payload, err := BuildDepositSubmit(DepositSubmitInput{
		Signer:    cfg.Signer,
		Factory:   cfg.DepositWalletFactory,
		Wallet:    cfg.Wallet,
		Calls:     calls,
		Nonce:     params.Nonce,
		Deadline:  deadline,
		Signature: sig,
		Metadata:  metadata,
	})
	if err != nil {
		return ExecuteResponse{}, err
	}
	return t.Submit(ctx, payload)
}

func submitProxy(
	ctx context.Context,
	t *Transport,
	cfg GaslessConfig,
	key *ecdsa.PrivateKey,
	calls []TransactionCall,
	metadata string,
) (ExecuteResponse, error) {
	// Proxy signs against the relay address, fetched via the unified params endpoint.
	params, err := t.FetchExecuteParams(ctx, addrHex(cfg.Signer), TransactionTypeProxy)
	if err != nil {
		return ExecuteResponse{}, err
	}
	to := cfg.ProxyFactory
	data, err := EncodeProxyCall(calls)
	if err != nil {
		return ExecuteResponse{}, err
	}
	relay := params.Address
	gasLimit := estimateProxyGasLimit(ctx, cfg, cfg.Signer, to, data)
	sig, err := Sign(TransactionTypeProxy, key, RelayRequest{
		Signer:   cfg.Signer,
		To:       to,
		Data:     data,
		GasFee:   big.NewInt(0),
		GasPrice: big.NewInt(0),
		GasLimit: gasLimit,
		Nonce:    params.Nonce,
		RelayHub: cfg.RelayHub,
		Relay:    relay,
	})
	if err != nil {
		return ExecuteResponse{}, err
	}
	payload, err := BuildProxySubmit(ProxySubmitInput{
		Signer:       cfg.Signer,
		ProxyFactory: to,
		Wallet:       cfg.Wallet,
		Data:         data,
		Nonce:        params.Nonce,
		GasLimit:     gasLimit,
		Signature:    sig,
		Relay:        relay,
		RelayHub:     cfg.RelayHub,
		Metadata:     metadata,
	})
	if err != nil {
		return ExecuteResponse{}, err
	}
	return t.Submit(ctx, payload)
}

func submitSafe(
	ctx context.Context,
	t *Transport,
	cfg GaslessConfig,
	key *ecdsa.PrivateKey,
	calls []TransactionCall,
	metadata string,
) (ExecuteResponse, error) {
	params, err := t.FetchExecuteParams(ctx, addrHex(cfg.Signer), TransactionTypeSafe)
	if err != nil {
		return ExecuteResponse{}, err
	}
	target, data, value, operation, err := resolveSafeCall(cfg.SafeMultisend, calls)
	if err != nil {
		return ExecuteResponse{}, err
	}
	sig, err := Sign(TransactionTypeSafe, key, RelayRequest{
		Wallet:    cfg.Wallet,
		To:        target,
		Data:      data,
		Value:     value,
		Operation: operation,
		Nonce:     params.Nonce,
		ChainID:   cfg.ChainID,
	})
	if err != nil {
		return ExecuteResponse{}, err
	}
	payload, err := BuildSafeSubmit(SafeSubmitInput{
		Signer:    cfg.Signer,
		Wallet:    cfg.Wallet,
		Target:    target,
		Data:      data,
		Value:     value,
		Operation: operation,
		Nonce:     params.Nonce,
		Signature: sig,
		Metadata:  metadata,
	})
	if err != nil {
		return ExecuteResponse{}, err
	}
	return t.Submit(ctx, payload)
}

// resolveSafeCall aggregates calls for a Safe submission: a single call is
// submitted directly; multiple calls are bundled via multiSend (delegatecall).
// Mirrors py's _resolve_safe_call / ts's aggregateSafeTransactionCalls.
func resolveSafeCall(
	safeMultisend common.Address,
	calls []TransactionCall,
) (target common.Address, data []byte, value *big.Int, operation uint8, err error) {
	if len(calls) == 1 {
		c := calls[0]
		if c.Value == nil {
			return common.Address{}, nil, nil, 0, fmt.Errorf("%w: safe call value", ErrNilValue)
		}
		return c.To, c.Data, c.Value, safeOperationCall, nil
	}
	encoded, err := EncodeSafeMultisendCall(calls)
	if err != nil {
		return common.Address{}, nil, nil, 0, err
	}
	return safeMultisend, encoded, big.NewInt(0), safeOperationDelegatecall, nil
}

func estimateProxyGasLimit(
	ctx context.Context,
	cfg GaslessConfig,
	from, to common.Address,
	data []byte,
) *big.Int {
	if cfg.GasEstimator == nil {
		return big.NewInt(ProxyDefaultGasLimit)
	}
	est, err := cfg.GasEstimator(ctx, from, to, data)
	if err != nil || est == 0 {
		return big.NewInt(ProxyDefaultGasLimit)
	}
	return new(big.Int).SetUint64(est)
}

// --- submit retry rules ---

// backoffWithJitter applies "equal jitter" to a retry delay: half fixed plus
// half random, yielding a value in [d/2, d]. This desynchronizes 429 retries
// across clients without ever shortening the delay below half the base.
func backoffWithJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	half := d / 2
	return half + time.Duration(rand.Float64()*float64(half))
}

var (
	walletBusyRE     = regexp.MustCompile(`(?i)wallet busy.*active action`)
	walletInflightRE = regexp.MustCompile(`(?i)wallet has in-flight action`)
	// batch nonce <submitted> does not match on-chain nonce <on-chain>
	nonceMismatchRE = regexp.MustCompile(
		`(?i)batch nonce\s+(\d+)\s+does not match on-chain nonce\s+(\d+)`,
	)
)

// isRetryableSubmitError reports whether a submit failure is worth retrying:
// rate limits, transient wallet contention, or a stale nonce (submitted < on-chain).
func isRetryableSubmitError(err error) bool {
	var apiErr *polyhttp.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusBadRequest:
		// already matched
	default:
		return false
	}
	msg := apiErr.Message
	if walletBusyRE.MatchString(msg) || walletInflightRE.MatchString(msg) {
		return true
	}
	if m := nonceMismatchRE.FindStringSubmatch(msg); m != nil {
		submitted, _ := strconv.ParseInt(m[1], 10, 64)
		onChain, _ := strconv.ParseInt(m[2], 10, 64)
		return submitted < onChain
	}
	return false
}
