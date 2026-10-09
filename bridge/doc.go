// Package bridge reads supported assets, quotes and transfer status, and
// explicitly registers deposit/withdrawal routing addresses across chains.
//
// Creating a withdrawal address does not sign or transfer tokens. The caller
// sends funds separately and can then poll GetStatus. Constructors perform no
// remote requests, and no operation automatically retries a registration POST.
//
// NewClient validates the configured HTTP(S) host. ChainID covers the full
// unsigned 64-bit range; BaseUnits is a canonical uint256 token quantity.
// Decimal retains every wire digit for monetary estimates and fee percentages.
// Token and recipient addresses remain strings for non-EVM network support.
package bridge
