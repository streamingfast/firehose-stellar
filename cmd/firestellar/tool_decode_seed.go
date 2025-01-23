package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stellar/go/keypair"
)

func NewToolDecodeSeedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-decode-seed <seed>",
		Short: "Tool to decode seed",
		Args:  cobra.ExactArgs(1),
		RunE:  toolDecodeSeedRunE,
	}
	return cmd
}

func toolDecodeSeedRunE(cmd *cobra.Command, args []string) error {
	seed := args[0]
	kp, err := keypair.ParseFull(seed)
	if err != nil {
		return err
	}
	fmt.Printf("Public Key: %s\n", kp.Address())
	return nil
}
