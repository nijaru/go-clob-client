// Package polyrelay implements Polymarket's gasless relayer transport: the
// signing schemes, payload shapes, and (in sibling files) the submit/poll
// lifecycle used to execute on-chain operations through Polymarket's relay
// server without the caller paying gas.
//
// The relayer is required for proxy, Safe, and deposit wallets, which can only
// act through relayed meta-transactions. EOA wallets may submit directly via
// go-ethereum, but the relayer is the sole execution path for the other wallet
// types and is therefore load-bearing for a general-purpose SDK.
//
// Three signing schemes are supported, one per wallet family. All produce a
// 65-byte secp256k1 signature; they differ in how the signed digest is derived:
//
//   - Proxy (POLY_PROXY): keccak256 of a packed "rlx:"-prefixed preimage,
//     signed via EIP-191 personal sign.
//   - Safe (POLY_GNOSIS_SAFE): an EIP-712 SafeTx whose digest is EIP-191
//     personal-signed (double-hashed), then the recovery byte is repacked to
//     the Safe signature-type encoding.
//   - DepositWallet: an EIP-712 Batch signed directly (standard EIP-712).
//
// The logic mirrors py-clob-client's polymarket._internal.actions.relayer
// modules byte-for-byte; test vectors are generated from that reference.
package polyrelay
