package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"go.uber.org/zap"
)

func NewToolSendPaymentAssetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-send-payment-asset <issuer-seed> <accound-dest-pk> <asset-code> <amount>",
		Short: "Tool to send payment",
		Args:  cobra.ExactArgs(4),
		RunE:  toolSendPaymentAssetRunE,
		Example: cli.Dedent(`
			firestellar tool-send-payment-asset GDKH5RILWVLADLB5LL44RL5EK4NWU3VDVF2E24S5CZU742YCTAR3WQN4 GCUPQ4MGWKLCQVANHN5WY7YJ26S6CEGNIDM757LEJDMAWBI5UDLKQBG6 edtokentest 10
		`),
	}

	cmd.Flags().Bool("double-send", false, "Send payment twice to the same destination account")

	return cmd
}

func toolSendPaymentAssetRunE(cmd *cobra.Command, args []string) error {
	issuerSeed := args[0]
	destination := args[1]
	assetCode := args[2]
	amount := args[3]

	issuer, err := keypair.ParseFull(issuerSeed)
	if err != nil {
		return fmt.Errorf("unable to parse issuer seed: %w", err)
	}

	client := horizonclient.DefaultTestNetClient

	destAccountRequest := horizonclient.AccountRequest{AccountID: destination}
	_, err = client.AccountDetail(destAccountRequest)
	if err != nil {
		return fmt.Errorf("unable to get destination account: %w", err)
	}

	request := horizonclient.AccountRequest{AccountID: issuer.Address()}
	issuerAccount, err := client.AccountDetail(request)
	if err != nil {
		return fmt.Errorf("unable to get issuer account: %w", err)
	}

	logger.Info("will send payment to destination account", zap.String("account", destination))

	customDollar := txnbuild.CreditAsset{Code: assetCode, Issuer: issuer.Address()}

	doubleSend := sflags.MustGetBool(cmd, "double-send")
	operations := []txnbuild.Operation{
		createPaymentOperation(destination, amount, customDollar),
	}

	// This is for testing purposes -> send the same payment twice
	if doubleSend {
		operations = append(operations, createPaymentOperation(destination, amount, customDollar))
	}

	tx, err := txnbuild.NewTransaction(
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
					Destination: destination,
					Asset:       customDollar,
					Amount:      amount,
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create transaction: %w", err)
	}

	signedTx, err := tx.Sign(network.TestNetworkPassphrase, issuer)
	if err != nil {
		return fmt.Errorf("unable to sign transaction: %w", err)
	}

	resp, err := client.SubmitTransaction(signedTx)
	if err != nil {
		return fmt.Errorf("Error sending payment: %s", err)
	}

	fmt.Println("Successful Transaction:")
	fmt.Println("Ledger:", resp.Ledger)
	fmt.Println("Transaction Hash:", resp.Hash)
	return nil
}
