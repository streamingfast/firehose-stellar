# Firestellar Tooling

## Create account

Run this command to crate a new account. Take note of the public and the private key.

```bash
firestellar tool-create-account
```

## Send payment for XLM

Run this command to send a payment from one account to another. You can run the `Create account` command twice to create two accounts. Then pass in the required information to the below command.

```bash
firestellar tool-send-payment <account-src-seed> <accound-dest-pk> <amount> # add --double-spend if you want to send the same amount twice
```

Once you have done that, the command will print the ledger entry and the transaction hash. You can validate it at `https://stellar.expert/explorer/testnet/{tx}`

## Send payment for a created asset

```bash
firestellar tool-send-payment-asset <account-src-seed> <accound-dest-pk> <issuer-seed> <asset-code> <amount>
```

## Issue new asset

1. Create 2 new accounts

```bash
# GDKH5RILWVLADLB5LL44RL5EK4NWU3VDVF2E24S5CZU742YCTAR3WQN4 SBCWFUMPBU35R4GRWCXHVP7CJ65GG7W5QF7EJ57ZVTMU2QXBETA4C3OK
firestellar tool-create-account
# GB7O7Y2TL24W5N5YR3HKZIDU57WAKTFDT2QCNHOWQPDLZQFRSNYIA6H5 SAYNB2VJFQWB2A5DENCU7WZSYD73S75OT4GNIBQUCN2FH332PFG3EGE5
firestellar tool-create-account
```

2. Issue new asset

```bash
firestellar tool-issue-asset <issuer-account-seed> <distributor-account-seed> <token-code>
```
