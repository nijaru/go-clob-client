// Package polyrelay implements the signing, call-encoding, and request-assembly
// pieces of Polymarket's gasless relayer — the components that turn a private
// key and a set of calls into a /submit request body the relayer accepts.
//
// It is required for proxy, Safe, and deposit wallets, which can only act
// through relayed meta-transactions. EOA wallets may submit on-chain ops
// directly via go-ethereum, but the relayer is the sole execution path for the
// other wallet families and is therefore load-bearing for a general SDK.
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
// Call encoders bundle many TransactionCalls into the single calldata blob the
// wallet signs (proxy factory tuple array, or Safe multiSend). Payload builders
// assemble the per-scheme /submit JSON body from a signature and typed inputs.
//
// The HTTP transport (nonce fetch, POST /submit, poll to confirmation,
// /deployed check) is not yet implemented; this package covers everything up to
// a submittable request.
package polyrelay
