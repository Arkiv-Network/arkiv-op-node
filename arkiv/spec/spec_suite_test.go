package spec_test

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func SuiteTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "StressTest Suite")
}

var pk *ecdsa.PrivateKey

// 0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf
var walletAddress common.Address
var client *ethclient.Client
var chainID *big.Int
var isLocalNode bool

var _ = BeforeSuite(func(ctx context.Context) {

	var err error
	pk, err = crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000001")
	Expect(err).To(Not(HaveOccurred()), "failed to convert private key")

	walletAddress = crypto.PubkeyToAddress(pk.PublicKey)

	isLocalNode = os.Getenv("LOCAL_NODE") == "true"

	nodeURL := os.Getenv("NODE_URL")
	Expect(nodeURL).To(Not(BeEmpty()), "NODE_URL is not set")

	client, err = ethclient.Dial(nodeURL)
	Expect(err).To(Not(HaveOccurred()), "failed to dial client")

	chainID, err = client.ChainID(ctx)
	Expect(err).To(Not(HaveOccurred()), "failed to get chain ID")

	if isLocalNode {
		rpcClient := client.Client()
		// Get the available accounts
		var accounts []common.Address
		err = rpcClient.CallContext(ctx, &accounts, "eth_accounts")
		Expect(err).To(Not(HaveOccurred()), "failed to get accounts")
		Expect(accounts).To(Not(BeEmpty()), "no accounts found")
		from := accounts[0]

		nonce, err := client.PendingNonceAt(ctx, from)
		Expect(err).To(Not(HaveOccurred()), "failed to get nonce")

		chainID, err := client.ChainID(ctx)
		Expect(err).To(Not(HaveOccurred()), "failed to get chain ID")

		tx := TransactionArgs{
			From:                 pointerOf(from),
			ChainID:              (*hexutil.Big)(chainID),
			Nonce:                (*hexutil.Uint64)(&nonce),
			MaxPriorityFeePerGas: (*hexutil.Big)(big.NewInt(1e9)), // 1 Gwei
			MaxFeePerGas:         (*hexutil.Big)(big.NewInt(5e9)), // 5 Gwei
			Gas:                  (*hexutil.Uint64)(pointerOf(uint64(2_800_000))),
			To:                   pointerOf(walletAddress), //
			Value:                (*hexutil.Big)(EthToWei(1)),
		}

		var txHash common.Hash

		err = rpcClient.CallContext(ctx, &txHash, "eth_sendTransaction", tx)
		Expect(err).To(Not(HaveOccurred()), "failed to send tx")

		_, err = bind.WaitMined(ctx, client, txHash)
		Expect(err).To(Not(HaveOccurred()), "failed to wait for tx")

	}

	balance, err := client.BalanceAt(ctx, walletAddress, nil)
	Expect(err).To(Not(HaveOccurred()), "failed to get balance")
	Expect(balance.String()).ToNot(Equal("0"), "balance of %s is zero", walletAddress.Hex())

})

func pointerOf[T any](v T) *T {
	return &v
}

type TransactionArgs struct {
	From                 *common.Address `json:"from"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big    `json:"value"`
	Nonce                *hexutil.Uint64 `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	// Issue detail: https://github.com/ethereum/go-ethereum/issues/15628
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`

	// Introduced by AccessListTxType transaction.
	AccessList *types.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.Big      `json:"chainId,omitempty"`

	// For BlobTxType
	BlobFeeCap *hexutil.Big  `json:"maxFeePerBlobGas"`
	BlobHashes []common.Hash `json:"blobVersionedHashes,omitempty"`

	// For BlobTxType transactions with blob sidecar
	Blobs       []kzg4844.Blob       `json:"blobs"`
	Commitments []kzg4844.Commitment `json:"commitments"`
	Proofs      []kzg4844.Proof      `json:"proofs"`

	// For SetCodeTxType
	AuthorizationList []types.SetCodeAuthorization `json:"authorizationList"`
}

func EthToWei(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(params.Ether))
}

// WaitMined waits for tx to be mined on the blockchain.
// It stops waiting when the context is canceled.
func waitMined(ctx context.Context, b *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	queryTicker := time.NewTicker(time.Millisecond * 20)
	defer queryTicker.Stop()

	for {
		receipt, err := b.TransactionReceipt(ctx, txHash)
		if err == nil {
			return receipt, nil
		}

		// Wait for the next round.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-queryTicker.C:
		}
	}
}
