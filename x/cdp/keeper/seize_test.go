package keeper_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	abci "github.com/tendermint/tendermint/abci/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/kava-labs/kava/app"
	"github.com/kava-labs/kava/x/auction"
	"github.com/kava-labs/kava/x/cdp/keeper"
	"github.com/kava-labs/kava/x/cdp/types"
)

type SeizeTestSuite struct {
	suite.Suite

	keeper       keeper.Keeper
	addrs        []sdk.AccAddress
	app          app.TestApp
	cdps         types.CDPs
	ctx          sdk.Context
	liquidations liquidationTracker
}

type liquidationTracker struct {
	xrp  []uint64
	btc  []uint64
	debt int64
}

func (suite *SeizeTestSuite) SetupTest() {
	tApp := app.NewTestApp()
	ctx := tApp.NewContext(true, abci.Header{Height: 1, Time: tmtime.Now()})
	coins := []sdk.Coins{}
	tracker := liquidationTracker{}

	for j := 0; j < 100; j++ {
		coins = append(coins, cs(c("btc", 100000000), c("xrp", 10000000000)))
	}
	_, addrs := app.GeneratePrivKeyAddressPairs(100)

	authGS := app.NewAuthGenState(
		addrs, coins)
	tApp.InitializeFromGenesisStates(
		authGS,
		NewPricefeedGenStateMulti(),
		NewCDPGenStateMulti(),
	)
	suite.ctx = ctx
	suite.app = tApp
	suite.keeper = tApp.GetCDPKeeper()
	suite.cdps = types.CDPs{}
	suite.addrs = addrs
	suite.liquidations = tracker
}

func (suite *SeizeTestSuite) createCdps() {
	tApp := app.NewTestApp()
	ctx := tApp.NewContext(true, abci.Header{Height: 1, Time: tmtime.Now()})
	cdps := make(types.CDPs, 100)
	_, addrs := app.GeneratePrivKeyAddressPairs(100)
	coins := []sdk.Coins{}
	tracker := liquidationTracker{}

	for j := 0; j < 100; j++ {
		coins = append(coins, cs(c("btc", 100000000), c("xrp", 10000000000)))
	}

	authGS := app.NewAuthGenState(
		addrs, coins)
	tApp.InitializeFromGenesisStates(
		authGS,
		NewPricefeedGenStateMulti(),
		NewCDPGenStateMulti(),
	)

	suite.ctx = ctx
	suite.app = tApp
	suite.keeper = tApp.GetCDPKeeper()
	randSource := rand.New(rand.NewSource(int64(777)))
	for j := 0; j < 100; j++ {
		collateral := "xrp"
		amount := 10000000000
		debt := simulation.RandIntBetween(randSource, 750000000, 1249000000)
		if j%2 == 0 {
			collateral = "btc"
			amount = 100000000
			debt = simulation.RandIntBetween(randSource, 2700000000, 5332000000)
			if debt >= 4000000000 {
				tracker.btc = append(tracker.btc, uint64(j+1))
				tracker.debt += int64(debt)
			}
		} else {
			if debt >= 1000000000 {
				tracker.xrp = append(tracker.xrp, uint64(j+1))
				tracker.debt += int64(debt)
			}
		}
		err := suite.keeper.AddCdp(suite.ctx, addrs[j], c(collateral, int64(amount)), c("usdx", int64(debt)), collateral+"-a")
		suite.NoError(err)
		c, f := suite.keeper.GetCDP(suite.ctx, collateral+"-a", uint64(j+1))
		suite.True(f)
		cdps[j] = c
	}

	suite.cdps = cdps
	suite.addrs = addrs
	suite.liquidations = tracker
}

func (suite *SeizeTestSuite) setPrice(price sdk.Dec, market string) {
	pfKeeper := suite.app.GetPriceFeedKeeper()

	pfKeeper.SetPrice(suite.ctx, sdk.AccAddress{}, market, price, suite.ctx.BlockTime().Add(time.Hour*3))
	err := pfKeeper.SetCurrentPrices(suite.ctx, market)
	suite.NoError(err)
	pp, err := pfKeeper.GetCurrentPrice(suite.ctx, market)
	suite.NoError(err)
	suite.Equal(price, pp.Price)
}

func (suite *SeizeTestSuite) TestSeizeCollateral() {
	suite.createCdps()
	sk := suite.app.GetSupplyKeeper()
	cdp, found := suite.keeper.GetCDP(suite.ctx, "xrp-a", uint64(2))
	suite.True(found)
	p := cdp.Principal.Amount
	cl := cdp.Collateral.Amount
	tpb := suite.keeper.GetTotalPrincipal(suite.ctx, "xrp-a", "usdx")
	err := suite.keeper.SeizeCollateral(suite.ctx, cdp)
	suite.NoError(err)
	tpa := suite.keeper.GetTotalPrincipal(suite.ctx, "xrp-a", "usdx")
	suite.Equal(tpb.Sub(tpa), p)
	auctionKeeper := suite.app.GetAuctionKeeper()
	_, found = auctionKeeper.GetAuction(suite.ctx, auction.DefaultNextAuctionID)
	suite.True(found)
	auctionMacc := sk.GetModuleAccount(suite.ctx, auction.ModuleName)
	suite.Equal(cs(c("debt", p.Int64()), c("xrp", cl.Int64())), auctionMacc.GetCoins())
	ak := suite.app.GetAccountKeeper()
	acc := ak.GetAccount(suite.ctx, suite.addrs[1])
	suite.Equal(p.Int64(), acc.GetCoins().AmountOf("usdx").Int64())
	err = suite.keeper.WithdrawCollateral(suite.ctx, suite.addrs[1], suite.addrs[1], c("xrp", 10), "xrp-a")
	suite.Require().True(errors.Is(err, types.ErrCdpNotFound))
}

func (suite *SeizeTestSuite) TestSeizeCollateralMultiDeposit() {
	suite.createCdps()
	sk := suite.app.GetSupplyKeeper()
	cdp, found := suite.keeper.GetCDP(suite.ctx, "xrp-a", uint64(2))
	suite.True(found)
	err := suite.keeper.DepositCollateral(suite.ctx, suite.addrs[1], suite.addrs[0], c("xrp", 6999000000), "xrp-a")
	suite.NoError(err)
	cdp, found = suite.keeper.GetCDP(suite.ctx, "xrp-a", uint64(2))
	suite.True(found)
	deposits := suite.keeper.GetDeposits(suite.ctx, cdp.ID)
	suite.Equal(2, len(deposits))
	p := cdp.Principal.Amount
	cl := cdp.Collateral.Amount
	tpb := suite.keeper.GetTotalPrincipal(suite.ctx, "xrp-a", "usdx")
	err = suite.keeper.SeizeCollateral(suite.ctx, cdp)
	suite.NoError(err)
	tpa := suite.keeper.GetTotalPrincipal(suite.ctx, "xrp-a", "usdx")
	suite.Equal(tpb.Sub(tpa), p)
	auctionMacc := sk.GetModuleAccount(suite.ctx, auction.ModuleName)
	suite.Equal(cs(c("debt", p.Int64()), c("xrp", cl.Int64())), auctionMacc.GetCoins())
	ak := suite.app.GetAccountKeeper()
	acc := ak.GetAccount(suite.ctx, suite.addrs[1])
	suite.Equal(p.Int64(), acc.GetCoins().AmountOf("usdx").Int64())
	err = suite.keeper.WithdrawCollateral(suite.ctx, suite.addrs[1], suite.addrs[1], c("xrp", 10), "xrp-a")
	suite.Require().True(errors.Is(err, types.ErrCdpNotFound))
}

func (suite *SeizeTestSuite) TestLiquidateCdps() {
	suite.createCdps()
	sk := suite.app.GetSupplyKeeper()
	acc := sk.GetModuleAccount(suite.ctx, types.ModuleName)
	originalXrpCollateral := acc.GetCoins().AmountOf("xrp")
	suite.setPrice(d("0.2"), "xrp:usd")
	p, found := suite.keeper.GetCollateral(suite.ctx, "xrp-a")
	suite.True(found)
	suite.keeper.LiquidateCdps(suite.ctx, "xrp:usd", "xrp-a", p.LiquidationRatio)
	acc = sk.GetModuleAccount(suite.ctx, types.ModuleName)
	finalXrpCollateral := acc.GetCoins().AmountOf("xrp")
	seizedXrpCollateral := originalXrpCollateral.Sub(finalXrpCollateral)
	xrpLiquidations := int(seizedXrpCollateral.Quo(i(10000000000)).Int64())
	suite.Equal(len(suite.liquidations.xrp), xrpLiquidations)
}

func (suite *SeizeTestSuite) TestApplyLiquidationPenalty() {
	penalty := suite.keeper.ApplyLiquidationPenalty(suite.ctx, "xrp-a", i(1000))
	suite.Equal(i(50), penalty)
	penalty = suite.keeper.ApplyLiquidationPenalty(suite.ctx, "btc-a", i(1000))
	suite.Equal(i(25), penalty)
	penalty = suite.keeper.ApplyLiquidationPenalty(suite.ctx, "xrp-a", i(675760172))
	suite.Equal(i(33788009), penalty)
	suite.Panics(func() { suite.keeper.ApplyLiquidationPenalty(suite.ctx, "lol-a", i(1000)) })
}

func TestSeizeTestSuite(t *testing.T) {
	suite.Run(t, new(SeizeTestSuite))
}
