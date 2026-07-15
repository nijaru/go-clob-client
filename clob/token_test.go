package clob

import (
	"bytes"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestTokenCallSelectors(t *testing.T) {
	t.Parallel()

	token := common.HexToAddress("0x1111111111111111111111111111111111111111")
	spender := common.HexToAddress("0x2222222222222222222222222222222222222222")
	recipient := common.HexToAddress("0x3333333333333333333333333333333333333333")

	approve, err := packERC20Approval(ERC20ApprovalRequest{
		TokenAddress:   token,
		SpenderAddress: spender,
		Amount:         big.NewInt(7),
	})
	if err != nil {
		t.Fatalf("packERC20Approval: %v", err)
	}
	transfer, err := packERC20Transfer(ERC20TransferRequest{
		TokenAddress:     token,
		RecipientAddress: recipient,
		Amount:           big.NewInt(8),
	})
	if err != nil {
		t.Fatalf("packERC20Transfer: %v", err)
	}
	approvalForAll, err := packERC1155ApprovalForAll(ERC1155ApprovalForAllRequest{
		TokenAddress:    token,
		OperatorAddress: spender,
		Approved:        true,
	})
	if err != nil {
		t.Fatalf("packERC1155ApprovalForAll: %v", err)
	}

	cases := []struct {
		name string
		want string
		got  []byte
	}{
		{"approve", "approve(address,uint256)", approve},
		{"transfer", "transfer(address,uint256)", transfer},
		{"setApprovalForAll", "setApprovalForAll(address,bool)", approvalForAll},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := selector(tc.want)
			if !bytes.Equal(tc.got[:4], want) {
				t.Fatalf("selector = %x, want %x", tc.got[:4], want)
			}
		})
	}
}

func TestTokenCallValidation(t *testing.T) {
	t.Parallel()

	tooBig := new(big.Int).Lsh(big.NewInt(1), 256)
	cases := []struct {
		name string
		fn   func() error
	}{
		{
			name: "approval nil",
			fn: func() error {
				_, err := packERC20Approval(ERC20ApprovalRequest{Amount: nil})
				return err
			},
		},
		{
			name: "approval negative",
			fn: func() error {
				_, err := packERC20Approval(ERC20ApprovalRequest{Amount: big.NewInt(-1)})
				return err
			},
		},
		{
			name: "approval overflow",
			fn: func() error {
				_, err := packERC20Approval(ERC20ApprovalRequest{Amount: tooBig})
				return err
			},
		},
		{
			name: "transfer negative",
			fn: func() error {
				_, err := packERC20Transfer(ERC20TransferRequest{Amount: big.NewInt(-1)})
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if _, err := packERC20Approval(ERC20ApprovalRequest{Amount: big.NewInt(-1)}); !errors.Is(
		err,
		ErrInvalidTokenAmount,
	) {
		t.Fatalf("invalid amount error = %v, want ErrInvalidTokenAmount", err)
	}

	max := MaxUint256()
	if max.BitLen() != 256 || max.Sign() < 0 {
		t.Fatalf("MaxUint256() = %s", max)
	}
	max.SetInt64(0)
	if MaxUint256().Sign() == 0 {
		t.Fatal("MaxUint256 returned shared mutable state")
	}
}

func TestGaslessTokenOperations(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	token := common.HexToAddress("0x1111111111111111111111111111111111111111")
	spender := common.HexToAddress("0x2222222222222222222222222222222222222222")
	recipient := common.HexToAddress("0x3333333333333333333333333333333333333333")

	cases := []struct {
		name string
		call func(*AuthenticatedClient) error
		want string
	}{
		{
			name: "approve",
			call: func(c *AuthenticatedClient) error {
				_, err := c.ApproveERC20Gasless(ctx, ERC20ApprovalRequest{
					TokenAddress: token, SpenderAddress: spender, Amount: big.NewInt(7),
				}, "approve")
				return err
			},
			want: "approve(address,uint256)",
		},
		{
			name: "approval-for-all",
			call: func(c *AuthenticatedClient) error {
				_, err := c.ApproveERC1155ForAllGasless(ctx, ERC1155ApprovalForAllRequest{
					TokenAddress: token, OperatorAddress: spender, Approved: true,
				}, "approval-for-all")
				return err
			},
			want: "setApprovalForAll(address,bool)",
		},
		{
			name: "transfer",
			call: func(c *AuthenticatedClient) error {
				_, err := c.TransferERC20Gasless(ctx, ERC20TransferRequest{
					TokenAddress: token, RecipientAddress: recipient, Amount: big.NewInt(8),
				}, "transfer")
				return err
			},
			want: "transfer(address,uint256)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured map[string]any
			srv := newGaslessCTFMockRelayer(t, &captured)
			t.Cleanup(srv.Close)
			client := newGaslessClient(t, SignatureTypePolyGnosisSafe, srv.URL)
			if err := tc.call(client); err != nil {
				t.Fatalf("gasless call: %v", err)
			}
			if got, _ := captured["to"].(string); got != token.Hex() {
				t.Fatalf("target = %q, want %q", got, token.Hex())
			}
			data, _ := captured["data"].(string)
			want := "0x" + common.Bytes2Hex(selector(tc.want))
			if len(data) < len(want) || data[:len(want)] != want {
				t.Fatalf("selector = %q, want %q", data, want)
			}
		})
	}
}

func TestDirectTokenOperationsRejectNonEOA(t *testing.T) {
	t.Parallel()

	client := newGaslessClient(t, SignatureTypePolyGnosisSafe, "http://127.0.0.1:1")
	_, err := client.ApproveERC20(t.Context(), ERC20ApprovalRequest{Amount: big.NewInt(1)})
	if err == nil || !errors.Is(err, ErrTokenOperationRequiresEOA) ||
		!strings.Contains(err.Error(), "use the gasless variant") {
		t.Fatalf("ApproveERC20 error = %v", err)
	}
}
