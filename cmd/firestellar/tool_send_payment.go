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

func NewToolSendPaymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-send-payment <source-account-seed> <destination-account> <amount>",
		Short: "Tool to send payment",
		Args:  cobra.ExactArgs(3),
		RunE:  toolSendPaymentRunE,
		Example: cli.Dedent(`
			# The source account is testnet account created. It should not be used for anything.
			firestellar tool-send-payment SDTVBHRE6N3VKFTVDDGPQHRNU7L26IXXPDP5AFQ6NPDHK5MYJLOQQGJ4 GCUPQ4MGWKLCQVANHN5WY7YJ26S6CEGNIDM757LEJDMAWBI5UDLKQBG6 1000
		`),
	}

	cmd.Flags().Bool("double-send", false, "Send payment twice to the same destination account")

	return cmd
}

func toolSendPaymentRunE(cmd *cobra.Command, args []string) error {
	// trx for account1 creation: 2dd5a4a183dc531fbd49d5d7b449e070e33a0f40227d61ddaaea0ed73e938691
	// account1 := "GA6QUJYL4AYEIZC7W6OEPGU3QDRK33WJP5WMMZLDZBUE7M3VWDF7LTTR"
	// source := "SDTVBHRE6N3VKFTVDDGPQHRNU7L26IXXPDP5AFQ6NPDHK5MYJLOQQGJ4"
	source := args[0]

	// trx for account 2 creation: c9eb7f8f7e5629c44dca331d1ee386939b92f9e973d36ccd452dadb230803675
	// destination := "GCUPQ4MGWKLCQVANHN5WY7YJ26S6CEGNIDM757LEJDMAWBI5UDLKQBG6"
	destination := args[1]
	// account2SK := "SDWOYBCKYGX5VRFT4L6WWTZMO3ZIGPNLTHKJZCI6DBZPIXT4YNSPV7O7"

	amount := args[2]

	client := horizonclient.DefaultTestNetClient

	// This is just a validation to make sure the destination account exists
	destAccountRequest := horizonclient.AccountRequest{AccountID: destination}
	_, err := client.AccountDetail(destAccountRequest)
	if err != nil {
		return fmt.Errorf("unable to get destination account: %w", err)
	}

	logger.Info("will send payment to destination account", zap.String("account", destination))

	// Load the source account from the private key
	sourceKP := keypair.MustParseFull(source)
	sourceAccountRequest := horizonclient.AccountRequest{AccountID: sourceKP.Address()}
	sourceAccount, err := client.AccountDetail(sourceAccountRequest)
	if err != nil {
		return fmt.Errorf("unable to get source account: %w", err)
	}

	doubleSend := sflags.MustGetBool(cmd, "double-send")
	operations := []txnbuild.Operation{
		createPaymentOperation(destination, amount, txnbuild.NativeAsset{}),
	}

	// This is for testing purposes -> send the same payment twice
	if doubleSend {
		operations = append(operations, createPaymentOperation(destination, amount, txnbuild.NativeAsset{}))
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions: txnbuild.Preconditions{
				TimeBounds: txnbuild.NewInfiniteTimeout(),
			},
			Operations: operations,
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create transaction: %w", err)
	}

	tx, err = tx.Sign(network.TestNetworkPassphrase, sourceKP)
	if err != nil {
		return fmt.Errorf("unable to sign transaction: %w", err)
	}

	resp, err := horizonclient.DefaultTestNetClient.SubmitTransaction(tx)
	if err != nil {
		return fmt.Errorf("unable to submit transaction: %w", err)
	}

	fmt.Println("Successful Transaction:")
	fmt.Println("Ledger:", resp.Ledger)
	fmt.Println("Transaction Hash:", resp.Hash)
	return nil
}

func createPaymentOperation(destination string, amount string, asset txnbuild.Asset) *txnbuild.Payment {
	return &txnbuild.Payment{
		Destination: destination,
		Amount:      amount,
		Asset:       asset,
	}
}
