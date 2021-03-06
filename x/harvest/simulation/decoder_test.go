package simulation

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/kv"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/kava-labs/kava/x/harvest/types"
)

func makeTestCodec() (cdc *codec.Codec) {
	cdc = codec.New()
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	types.RegisterCodec(cdc)
	return
}

func TestDecodeDistributionStore(t *testing.T) {
	cdc := makeTestCodec()

	prevBlockTime := time.Now().UTC()
	deposit := types.NewDeposit(sdk.AccAddress("test"), sdk.NewCoin("bnb", sdk.NewInt(1)))
	claim := types.NewClaim(sdk.AccAddress("test"), "bnb", sdk.NewCoin("hard", sdk.NewInt(100)), "stake")

	kvPairs := kv.Pairs{
		kv.Pair{Key: []byte(types.PreviousBlockTimeKey), Value: cdc.MustMarshalBinaryBare(prevBlockTime)},
		kv.Pair{Key: []byte(types.PreviousDelegationDistributionKey), Value: cdc.MustMarshalBinaryBare(prevBlockTime)},
		kv.Pair{Key: []byte(types.DepositsKeyPrefix), Value: cdc.MustMarshalBinaryBare(deposit)},
		kv.Pair{Key: []byte(types.ClaimsKeyPrefix), Value: cdc.MustMarshalBinaryBare(claim)},
		kv.Pair{Key: []byte{0x99}, Value: []byte{0x99}},
	}

	tests := []struct {
		name        string
		expectedLog string
	}{
		{"PreviousBlockTime", fmt.Sprintf("%s\n%s", prevBlockTime, prevBlockTime)},
		{"PreviousDistributionTime", fmt.Sprintf("%s\n%s", prevBlockTime, prevBlockTime)},
		{"Deposit", fmt.Sprintf("%s\n%s", deposit, deposit)},
		{"Claim", fmt.Sprintf("%s\n%s", claim, claim)},
		{"other", ""},
	}
	for i, tt := range tests {
		i, tt := i, tt
		t.Run(tt.name, func(t *testing.T) {
			switch i {
			case len(tests) - 1:
				require.Panics(t, func() { DecodeStore(cdc, kvPairs[i], kvPairs[i]) }, tt.name)
			default:
				require.Equal(t, tt.expectedLog, DecodeStore(cdc, kvPairs[i], kvPairs[i]), tt.name)
			}
		})
	}
}
