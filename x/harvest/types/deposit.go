package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Deposit defines an amount of coins deposited into a harvest module account
type Deposit struct {
	Depositor sdk.AccAddress `json:"depositor" yaml:"depositor"`
	Amount    sdk.Coin       `json:"amount" yaml:"amount"`
}

// NewDeposit returns a new deposit
func NewDeposit(depositor sdk.AccAddress, amount sdk.Coin) Deposit {
	return Deposit{
		Depositor: depositor,
		Amount:    amount,
	}
}
