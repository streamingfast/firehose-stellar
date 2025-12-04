package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"go.uber.org/zap"
)

func NewToolIssueAssetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-issue-asset <issuer-account-seed> <distributor-account-seed> <token-code>",
		Short: "Tool to issue asset",
		Args:  cobra.ExactArgs(3),
		RunE:  toolIssueAssetRunE,
	}

	return cmd
}

func toolIssueAssetRunE(cmd *cobra.Command, args []string) error {
	client := horizonclient.DefaultTestNetClient

	issuerSeed := args[0]
	distributorSeed := args[1]

	// Keys for accounts to issue and distribute the new asset.
	issuer, err := keypair.ParseFull(issuerSeed)
	if err != nil {
		return fmt.Errorf("unable to parse issuer seed: %w", err)
	}

	distributor, err := keypair.ParseFull(distributorSeed)
	if err != nil {
		return fmt.Errorf("unable to parse distributor seed: %w", err)
	}

	request := horizonclient.AccountRequest{AccountID: issuer.Address()}
	issuerAccount, err := client.AccountDetail(request)
	if err != nil {
		return fmt.Errorf("unable to get issuer account: %w", err)
	}

	request = horizonclient.AccountRequest{AccountID: distributor.Address()}
	distributorAccount, err := client.AccountDetail(request)
	if err != nil {
		return fmt.Errorf("unable to get distributor account: %w", err)
	}

	customDollar := txnbuild.CreditAsset{Code: args[2], Issuer: issuer.Address()}

	// First, the receiving (distribution) account must trust the asset from the issuer
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &txnbuild.SimpleAccount{
				AccountID: distributorAccount.AccountID,
				Sequence:  distributorAccount.Sequence,
			},
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions: txnbuild.Preconditions{
				TimeBounds: txnbuild.NewInfiniteTimeout(),
			},
			Operations: []txnbuild.Operation{
				&txnbuild.ChangeTrust{
					Line: txnbuild.ChangeTrustAssetWrapper{
						Asset: customDollar,
					},
					Limit: "5000",
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create transaction: %w", err)
	}

	signedTx, err := tx.Sign(network.TestNetworkPassphrase, distributor)
	if err != nil {
		return fmt.Errorf("unable to sign transaction: %w", err)
	}
	signedTxHash, err := signedTx.HashHex(network.TestNetworkPassphrase)
	if err != nil {
		return fmt.Errorf("unable to hash transaction: %w", err)
	}
	logger.Info("transaction", zap.String("hash", signedTxHash), zap.Uint64("sequence", uint64(signedTx.SequenceNumber())))

	resp, err := client.SubmitTransaction(signedTx)
	if err != nil {
		return fmt.Errorf("unable to submit transaction: %w", err)
	}

	fmt.Printf("Trust Transaction Hash: %s\n", resp.Hash)

	// Second, the issuing account actually sends a payment using the asset
	tx, err = txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &txnbuild.SimpleAccount{
				AccountID: issuerAccount.AccountID,
				Sequence:  issuerAccount.Sequence,
			},
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions: txnbuild.Preconditions{
				TimeBounds: txnbuild.NewInfiniteTimeout(),
			},
			Operations: []txnbuild.Operation{
				&txnbuild.Payment{
					Destination: distributor.Address(),
					Asset:       customDollar,
					Amount:      "10",
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create transaction: %w", err)
	}

	signedTx, err = tx.Sign(network.TestNetworkPassphrase, issuer)
	if err != nil {
		return fmt.Errorf("unable to sign transaction: %w", err)
	}

	resp, err = client.SubmitTransaction(signedTx)
	if err != nil {
		return fmt.Errorf("Error sending payment: %s", err)
	}

	fmt.Printf("Pay Transaction Hash: %s\n", resp.Hash)
	return nil
}
