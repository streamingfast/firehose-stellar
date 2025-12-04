package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/keypair"
	"go.uber.org/zap"
)

func NewToolCreateAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-create-account",
		Short: "Tool to create account",
		Args:  cobra.ExactArgs(0),
		RunE:  toolCreateAccountRunE,
	}

	return cmd
}

func toolCreateAccountRunE(cmd *cobra.Command, args []string) (err error) {
	pair, err := keypair.Random()
	if err != nil {
		return fmt.Errorf("unable to generate keypair: %w", err)
	}

	logger.Info("Generated keypair", zap.String("public_key", pair.Address()), zap.String("secret_key", pair.Seed()))

	address := pair.Address()
	resp, err := http.Get("https://friendbot.stellar.org/?addr=" + address)
	if err != nil {
		return fmt.Errorf("unable to fund account: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read response: %w", err)
	}
	logger.Info("Funded account", zap.String("response", string(body)))

	return nil
}
