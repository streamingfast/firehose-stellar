package stellar

import (
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// Payment builds a payment operation. Pass txnbuild.NativeAsset{} for XLM
// or txnbuild.CreditAsset{Code, Issuer} for a custom asset.
func Payment(destination, amount string, asset txnbuild.Asset) *txnbuild.Payment {
	return &txnbuild.Payment{Destination: destination, Amount: amount, Asset: asset}
}

// CreateAccount builds a CreateAccount operation, used for funding a new
// account from an existing one (the alternative to friendbot).
func CreateAccount(destination, startingBalance string) *txnbuild.CreateAccount {
	return &txnbuild.CreateAccount{Destination: destination, Amount: startingBalance}
}

// ChangeTrust builds a trustline-creation operation for the given asset.
func ChangeTrust(asset txnbuild.Asset, limit string) *txnbuild.ChangeTrust {
	wrap := txnbuild.ChangeTrustAssetWrapper{Asset: asset.(txnbuild.CreditAsset)}
	return &txnbuild.ChangeTrust{Line: wrap, Limit: limit}
}

// ManageData stores a key/value pair on an account. Useful for "boring"
// scenarios that exercise no asset movement.
func ManageData(name, value string) *txnbuild.ManageData {
	return &txnbuild.ManageData{Name: name, Value: []byte(value)}
}

// AccountMerge merges the source account into the destination, sweeping its
// XLM balance.
func AccountMerge(destination string) *txnbuild.AccountMerge {
	return &txnbuild.AccountMerge{Destination: destination}
}

// CreditAsset is a convenience constructor.
func CreditAsset(code string, issuer *keypair.Full) txnbuild.CreditAsset {
	return txnbuild.CreditAsset{Code: code, Issuer: issuer.Address()}
}
