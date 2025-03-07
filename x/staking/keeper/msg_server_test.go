package keeper_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	sdkstaking "github.com/cosmos/cosmos-sdk/x/staking/types"
	simapp "github.com/iqlusioninc/liquidity-staking-module/app"
	"github.com/iqlusioninc/liquidity-staking-module/x/staking/keeper"
	"github.com/iqlusioninc/liquidity-staking-module/x/staking/teststaking"
	"github.com/iqlusioninc/liquidity-staking-module/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestTokenizeSharesAndRedeemTokens(t *testing.T) {
	_, app, ctx := createTestInput(t)

	liquidStakingCapStrict := sdk.ZeroDec()
	liquidStakingCapConservative := sdk.MustNewDecFromStr("0.8")
	liquidStakingCapDisabled := sdk.OneDec()

	validatorBondStrict := sdk.OneDec()
	validatorBondConservative := sdk.NewDec(10)
	validatorBondDisabled := sdk.NewDec(-1)

	testCases := []struct {
		name                          string
		vestingAmount                 sdk.Int
		delegationAmount              sdk.Int
		tokenizeShareAmount           sdk.Int
		redeemAmount                  sdk.Int
		targetVestingDelAfterShare    sdk.Int
		targetVestingDelAfterRedeem   sdk.Int
		globalLiquidStakingCap        sdk.Dec
		slashFactor                   sdk.Dec
		validatorLiquidStakingCap     sdk.Dec
		validatorBondFactor           sdk.Dec
		validatorBondDelegation       bool
		validatorBondDelegatorIndex   int
		delegatorIsLSTP               bool
		expTokenizeErr                bool
		expRedeemErr                  bool
		prevAccountDelegationExists   bool
		recordAccountDelegationExists bool
	}{
		{
			name:                          "full amount tokenize and redeem",
			vestingAmount:                 sdk.NewInt(0),
			delegationAmount:              app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:           app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			redeemAmount:                  app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			slashFactor:                   sdk.ZeroDec(),
			globalLiquidStakingCap:        liquidStakingCapDisabled,
			validatorLiquidStakingCap:     liquidStakingCapDisabled,
			validatorBondFactor:           validatorBondDisabled,
			validatorBondDelegation:       false,
			expTokenizeErr:                false,
			expRedeemErr:                  false,
			prevAccountDelegationExists:   false,
			recordAccountDelegationExists: false,
		},
		{
			name:                          "full amount tokenize and partial redeem",
			vestingAmount:                 sdk.NewInt(0),
			delegationAmount:              app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:           app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			redeemAmount:                  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                   sdk.ZeroDec(),
			globalLiquidStakingCap:        liquidStakingCapDisabled,
			validatorLiquidStakingCap:     liquidStakingCapDisabled,
			validatorBondFactor:           validatorBondDisabled,
			validatorBondDelegation:       false,
			expTokenizeErr:                false,
			expRedeemErr:                  false,
			prevAccountDelegationExists:   false,
			recordAccountDelegationExists: true,
		},
		{
			name:                          "partial amount tokenize and full redeem",
			vestingAmount:                 sdk.NewInt(0),
			delegationAmount:              app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:           app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                   sdk.ZeroDec(),
			globalLiquidStakingCap:        liquidStakingCapDisabled,
			validatorLiquidStakingCap:     liquidStakingCapDisabled,
			validatorBondFactor:           validatorBondDisabled,
			validatorBondDelegation:       false,
			expTokenizeErr:                false,
			expRedeemErr:                  false,
			prevAccountDelegationExists:   true,
			recordAccountDelegationExists: false,
		},
		{
			name:                          "tokenize and redeem with slash",
			vestingAmount:                 sdk.NewInt(0),
			delegationAmount:              app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:           app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			redeemAmount:                  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                   sdk.MustNewDecFromStr("0.1"),
			globalLiquidStakingCap:        liquidStakingCapDisabled,
			validatorLiquidStakingCap:     liquidStakingCapDisabled,
			validatorBondFactor:           validatorBondDisabled,
			validatorBondDelegation:       false,
			expTokenizeErr:                false,
			expRedeemErr:                  false,
			prevAccountDelegationExists:   false,
			recordAccountDelegationExists: true,
		},
		{
			name:                      "over tokenize",
			vestingAmount:             sdk.NewInt(0),
			delegationAmount:          app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:       app.StakingKeeper.TokensFromConsensusPower(ctx, 30),
			redeemAmount:              app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			slashFactor:               sdk.ZeroDec(),
			globalLiquidStakingCap:    liquidStakingCapDisabled,
			validatorLiquidStakingCap: liquidStakingCapDisabled,
			validatorBondFactor:       validatorBondDisabled,
			validatorBondDelegation:   false,
			expTokenizeErr:            true,
			expRedeemErr:              false,
		},
		{
			name:                      "over redeem",
			vestingAmount:             sdk.NewInt(0),
			delegationAmount:          app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:       app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			redeemAmount:              app.StakingKeeper.TokensFromConsensusPower(ctx, 40),
			slashFactor:               sdk.ZeroDec(),
			globalLiquidStakingCap:    liquidStakingCapDisabled,
			validatorLiquidStakingCap: liquidStakingCapDisabled,
			validatorBondFactor:       validatorBondDisabled,
			validatorBondDelegation:   false,
			expTokenizeErr:            false,
			expRedeemErr:              true,
		},
		{
			name:                        "vesting account tokenize share failure",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapDisabled,
			validatorLiquidStakingCap:   liquidStakingCapDisabled,
			validatorBondFactor:         validatorBondDisabled,
			validatorBondDelegation:     false,
			expTokenizeErr:              true,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "vesting account tokenize share success",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapDisabled,
			validatorLiquidStakingCap:   liquidStakingCapDisabled,
			validatorBondFactor:         validatorBondDisabled,
			validatorBondDelegation:     false,
			expTokenizeErr:              false,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "try tokenize share for a validator-bond delegation",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapDisabled,
			validatorLiquidStakingCap:   liquidStakingCapDisabled,
			validatorBondFactor:         validatorBondConservative,
			validatorBondDelegation:     true,
			validatorBondDelegatorIndex: 1,
			expTokenizeErr:              true,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "strict validator-bond - tokenization fails",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapDisabled,
			validatorLiquidStakingCap:   liquidStakingCapDisabled,
			validatorBondFactor:         validatorBondStrict,
			validatorBondDelegation:     false,
			expTokenizeErr:              true,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "conservative validator-bond - successful tokenization",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapDisabled,
			validatorLiquidStakingCap:   liquidStakingCapDisabled,
			validatorBondFactor:         validatorBondConservative,
			validatorBondDelegation:     true,
			validatorBondDelegatorIndex: 0,
			expTokenizeErr:              false,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "strict global liquid staking cap - tokenization fails",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapStrict,
			validatorLiquidStakingCap:   liquidStakingCapDisabled,
			validatorBondFactor:         validatorBondDisabled,
			validatorBondDelegation:     true,
			validatorBondDelegatorIndex: 0,
			expTokenizeErr:              true,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "conservative global liquid staking cap - successful tokenization",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapConservative,
			validatorLiquidStakingCap:   liquidStakingCapDisabled,
			validatorBondFactor:         validatorBondDisabled,
			validatorBondDelegation:     true,
			validatorBondDelegatorIndex: 0,
			expTokenizeErr:              false,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "strict validator liquid staking cap - tokenization fails",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapDisabled,
			validatorLiquidStakingCap:   liquidStakingCapStrict,
			validatorBondFactor:         validatorBondDisabled,
			validatorBondDelegation:     true,
			validatorBondDelegatorIndex: 0,
			expTokenizeErr:              true,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "conservative validator liquid staking cap - successful tokenization",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapDisabled,
			validatorLiquidStakingCap:   liquidStakingCapConservative,
			validatorBondFactor:         validatorBondDisabled,
			validatorBondDelegation:     true,
			validatorBondDelegatorIndex: 0,
			expTokenizeErr:              false,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "all caps set conservatively - successful tokenize share",
			vestingAmount:               app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapConservative,
			validatorLiquidStakingCap:   liquidStakingCapConservative,
			validatorBondFactor:         validatorBondConservative,
			validatorBondDelegation:     true,
			validatorBondDelegatorIndex: 0,
			expTokenizeErr:              false,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
		{
			name:                        "delegator is a liquid staking provider - accounting should not update",
			vestingAmount:               sdk.ZeroInt(),
			delegationAmount:            app.StakingKeeper.TokensFromConsensusPower(ctx, 20),
			tokenizeShareAmount:         app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			redeemAmount:                app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterShare:  app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			targetVestingDelAfterRedeem: app.StakingKeeper.TokensFromConsensusPower(ctx, 10),
			slashFactor:                 sdk.ZeroDec(),
			globalLiquidStakingCap:      liquidStakingCapConservative,
			validatorLiquidStakingCap:   liquidStakingCapConservative,
			validatorBondFactor:         validatorBondConservative,
			delegatorIsLSTP:             true,
			validatorBondDelegation:     true,
			validatorBondDelegatorIndex: 0,
			expTokenizeErr:              false,
			expRedeemErr:                false,
			prevAccountDelegationExists: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, app, ctx = createTestInput(t)
			addrs := simapp.AddTestAddrs(app, ctx, 2, app.StakingKeeper.TokensFromConsensusPower(ctx, 10000))
			addrAcc1, addrAcc2 := addrs[0], addrs[1]
			addrVal1, addrVal2 := sdk.ValAddress(addrAcc1), sdk.ValAddress(addrAcc2)

			// Create ICA module account
			icaAccountAddress := createICAAccount(app, ctx, "ica-module-account")

			// Fund module account
			delegationCoin := sdk.NewCoin(app.StakingKeeper.BondDenom(ctx), tc.delegationAmount)
			err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(delegationCoin))
			require.NoError(t, err)
			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, icaAccountAddress, sdk.NewCoins(delegationCoin))
			require.NoError(t, err)

			// set the delegator address depending on whether the delegator should be a liquid staking provider
			delegatorAccount := addrAcc2
			if tc.delegatorIsLSTP {
				delegatorAccount = icaAccountAddress
			}

			// set validator bond factor and global liquid staking cap
			params := app.StakingKeeper.GetParams(ctx)
			params.ValidatorBondFactor = tc.validatorBondFactor
			params.GlobalLiquidStakingCap = tc.globalLiquidStakingCap
			params.ValidatorLiquidStakingCap = tc.validatorLiquidStakingCap
			app.StakingKeeper.SetParams(ctx, params)

			// set the total liquid staked tokens
			app.StakingKeeper.SetTotalLiquidStakedTokens(ctx, sdk.ZeroInt())

			if !tc.vestingAmount.IsZero() {
				// create vesting account
				pubkey := secp256k1.GenPrivKey().PubKey()
				baseAcc := authtypes.NewBaseAccount(addrAcc2, pubkey, 0, 0)
				initialVesting := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, tc.vestingAmount))
				baseVestingWithCoins := vestingtypes.NewBaseVestingAccount(baseAcc, initialVesting, ctx.BlockTime().Unix()+86400*365)
				delayedVestingAccount := vestingtypes.NewDelayedVestingAccountRaw(baseVestingWithCoins)
				app.AccountKeeper.SetAccount(ctx, delayedVestingAccount)
			}

			pubKeys := simapp.CreateTestPubKeys(2)
			pk1, pk2 := pubKeys[0], pubKeys[1]

			// Create Validators and Delegation
			val1 := teststaking.NewValidator(t, addrVal1, pk1)
			val1.Status = sdkstaking.Bonded
			app.StakingKeeper.SetValidator(ctx, val1)
			app.StakingKeeper.SetValidatorByPowerIndex(ctx, val1)
			err = app.StakingKeeper.SetValidatorByConsAddr(ctx, val1)
			require.NoError(t, err)

			val2 := teststaking.NewValidator(t, addrVal2, pk2)
			val2.Status = sdkstaking.Bonded
			app.StakingKeeper.SetValidator(ctx, val2)
			app.StakingKeeper.SetValidatorByPowerIndex(ctx, val2)
			err = app.StakingKeeper.SetValidatorByConsAddr(ctx, val2)
			require.NoError(t, err)

			// Delegate from both the main delegator as well as a random account so there is a
			// non-zero delegation after redemption
			err = delegateCoinsFromAccount(ctx, app, delegatorAccount, tc.delegationAmount, val1)
			require.NoError(t, err)

			// apply TM updates
			applyValidatorSetUpdates(t, ctx, app.StakingKeeper, -1)

			_, found := app.StakingKeeper.GetLiquidDelegation(ctx, delegatorAccount, addrVal1)
			require.True(t, found, "delegation not found after delegate")

			lastRecordID := app.StakingKeeper.GetLastTokenizeShareRecordID(ctx)
			oldValidator, found := app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
			require.True(t, found)

			msgServer := keeper.NewMsgServerImpl(app.StakingKeeper)
			if tc.validatorBondDelegation {
				err := delegateCoinsFromAccount(ctx, app, addrs[tc.validatorBondDelegatorIndex], tc.delegationAmount, val1)
				require.NoError(t, err)
				_, err = msgServer.ValidatorBond(sdk.WrapSDKContext(ctx), &types.MsgValidatorBond{
					DelegatorAddress: addrs[tc.validatorBondDelegatorIndex].String(),
					ValidatorAddress: addrVal1.String(),
				})
				require.NoError(t, err)
			}

			resp, err := msgServer.TokenizeShares(sdk.WrapSDKContext(ctx), &types.MsgTokenizeShares{
				DelegatorAddress:    delegatorAccount.String(),
				ValidatorAddress:    addrVal1.String(),
				Amount:              sdk.NewCoin(app.StakingKeeper.BondDenom(ctx), tc.tokenizeShareAmount),
				TokenizedShareOwner: delegatorAccount.String(),
			})
			if tc.expTokenizeErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// check last record id increase
			require.Equal(t, lastRecordID+1, app.StakingKeeper.GetLastTokenizeShareRecordID(ctx))

			// ensure validator's total tokens is consistent
			newValidator, found := app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
			require.True(t, found)
			require.Equal(t, oldValidator.Tokens, newValidator.Tokens)

			// if the delegator was not a provider, check that the total liquid staked and validator liquid shares increased
			totalLiquidTokensAfterTokenization := app.StakingKeeper.GetTotalLiquidStakedTokens(ctx)
			validatorLiquidSharesAfterTokenization := newValidator.TotalLiquidShares
			if !tc.delegatorIsLSTP {
				require.Equal(t, tc.tokenizeShareAmount.String(), totalLiquidTokensAfterTokenization.String(), "total liquid tokens after tokenization")
				require.Equal(t, tc.tokenizeShareAmount.String(), validatorLiquidSharesAfterTokenization.TruncateInt().String(), "validator liquid shares after tokenization")
			} else {
				require.True(t, totalLiquidTokensAfterTokenization.IsZero(), "zero liquid tokens after tokenization")
				require.True(t, validatorLiquidSharesAfterTokenization.IsZero(), "zero liquid validator shares after tokenization")
			}

			if tc.vestingAmount.IsPositive() {
				acc := app.AccountKeeper.GetAccount(ctx, addrAcc2)
				vestingAcc := acc.(vesting.VestingAccount)
				require.Equal(t, vestingAcc.GetDelegatedVesting().AmountOf(app.StakingKeeper.BondDenom(ctx)).String(), tc.targetVestingDelAfterShare.String())
			}

			if tc.prevAccountDelegationExists {
				_, found = app.StakingKeeper.GetLiquidDelegation(ctx, delegatorAccount, addrVal1)
				require.True(t, found, "delegation found after partial tokenize share")
			} else {
				_, found = app.StakingKeeper.GetLiquidDelegation(ctx, delegatorAccount, addrVal1)
				require.False(t, found, "delegation found after full tokenize share")
			}

			shareToken := app.BankKeeper.GetBalance(ctx, delegatorAccount, resp.Amount.Denom)
			require.Equal(t, resp.Amount, shareToken)
			_, found = app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
			require.True(t, found, true, "validator not found")

			records := app.StakingKeeper.GetAllTokenizeShareRecords(ctx)
			require.Len(t, records, 1)
			delegation, found := app.StakingKeeper.GetLiquidDelegation(ctx, records[0].GetModuleAddress(), addrVal1)
			require.True(t, found, "delegation not found from tokenize share module account after tokenize share")

			// slash before redeem
			slashedTokens := sdk.ZeroInt()
			redeemedShares := tc.redeemAmount
			redeemedTokens := tc.redeemAmount
			if tc.slashFactor.IsPositive() {
				consAddr, err := val1.GetConsAddr()
				require.NoError(t, err)
				ctx = ctx.WithBlockHeight(100)
				val1, found = app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
				require.True(t, found)
				power := app.StakingKeeper.TokensToConsensusPower(ctx, val1.Tokens)
				app.StakingKeeper.Slash(ctx, consAddr, 10, power, tc.slashFactor, 0)
				slashedTokens = sdk.NewDecFromInt(val1.Tokens).Mul(tc.slashFactor).TruncateInt()

				val1, _ := app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
				redeemedShares = delegation.Shares.Mul(sdk.NewDecFromInt(tc.redeemAmount)).QuoInt(shareToken.Amount).TruncateInt()
				redeemedTokens = val1.TokensFromShares(sdk.NewDecFromInt(redeemedShares)).TruncateInt()
			}

			// get deletagor balance and delegation
			bondDenomAmountBefore := app.BankKeeper.GetBalance(ctx, delegatorAccount, app.StakingKeeper.BondDenom(ctx))
			val1, found = app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
			require.True(t, found)
			delegation, found = app.StakingKeeper.GetLiquidDelegation(ctx, delegatorAccount, addrVal1)
			if !found {
				delegation = types.Delegation{Shares: sdk.ZeroDec()}
			}
			delAmountBefore := val1.TokensFromShares(delegation.Shares)
			oldValidator, found = app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
			require.True(t, found)

			_, err = msgServer.RedeemTokens(sdk.WrapSDKContext(ctx), &types.MsgRedeemTokensforShares{
				DelegatorAddress: delegatorAccount.String(),
				Amount:           sdk.NewCoin(resp.Amount.Denom, tc.redeemAmount),
			})
			if tc.expRedeemErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// ensure validator's total tokens is consistent
			newValidator, found = app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
			require.True(t, found)
			require.Equal(t, oldValidator.Tokens, newValidator.Tokens)

			// if the delegator was not a liuqid staking provider, check that the total liquid staked
			// and total liquid shares decreased
			totalLiquidTokensAfterRedemption := app.StakingKeeper.GetTotalLiquidStakedTokens(ctx)
			validatorLiquidSharesAfterRedemption := newValidator.TotalLiquidShares
			expectedLiquidTokens := totalLiquidTokensAfterTokenization.Sub(redeemedTokens).Sub(slashedTokens)
			expectedLiquidShares := validatorLiquidSharesAfterTokenization.Sub(sdk.NewDecFromInt(redeemedShares))
			if !tc.delegatorIsLSTP {
				require.Equal(t, expectedLiquidTokens.String(), totalLiquidTokensAfterRedemption.String(), "total liquid tokens after redemption")
				require.Equal(t, expectedLiquidShares.String(), validatorLiquidSharesAfterRedemption.String(), "validator liquid shares after tokenization")
			} else {
				require.True(t, totalLiquidTokensAfterRedemption.IsZero(), "zero liquid tokens after redemption")
				require.True(t, validatorLiquidSharesAfterRedemption.IsZero(), "zero liquid validator shares after redemption")
			}

			if tc.vestingAmount.IsPositive() {
				acc := app.AccountKeeper.GetAccount(ctx, addrAcc2)
				vestingAcc := acc.(vesting.VestingAccount)
				require.Equal(t, vestingAcc.GetDelegatedVesting().AmountOf(app.StakingKeeper.BondDenom(ctx)).String(), tc.targetVestingDelAfterRedeem.String())
			}

			expectedDelegatedShares := sdk.NewDecFromInt(tc.delegationAmount.Sub(tc.tokenizeShareAmount).Add(tc.redeemAmount))
			delegation, found = app.StakingKeeper.GetLiquidDelegation(ctx, delegatorAccount, addrVal1)
			require.True(t, found, "delegation not found after redeem tokens")
			require.Equal(t, delegatorAccount.String(), delegation.DelegatorAddress)
			require.Equal(t, addrVal1.String(), delegation.ValidatorAddress)
			require.Equal(t, expectedDelegatedShares, delegation.Shares, "delegation shares after redeem")

			// check delegator balance is not changed
			bondDenomAmountAfter := app.BankKeeper.GetBalance(ctx, delegatorAccount, app.StakingKeeper.BondDenom(ctx))
			require.Equal(t, bondDenomAmountAfter.Amount.String(), bondDenomAmountBefore.Amount.String())

			// get delegation amount is changed correctly
			val1, found = app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
			require.True(t, found)
			delegation, found = app.StakingKeeper.GetLiquidDelegation(ctx, delegatorAccount, addrVal1)
			if !found {
				delegation = types.Delegation{Shares: sdk.ZeroDec()}
			}
			delAmountAfter := val1.TokensFromShares(delegation.Shares)
			require.Equal(t, delAmountAfter.String(), delAmountBefore.Add(sdk.NewDecFromInt(tc.redeemAmount).Mul(sdk.OneDec().Sub(tc.slashFactor))).String())

			shareToken = app.BankKeeper.GetBalance(ctx, delegatorAccount, resp.Amount.Denom)
			require.Equal(t, shareToken.Amount.String(), tc.tokenizeShareAmount.Sub(tc.redeemAmount).String())
			_, found = app.StakingKeeper.GetLiquidValidator(ctx, addrVal1)
			require.True(t, found, true, "validator not found")

			if tc.recordAccountDelegationExists {
				_, found = app.StakingKeeper.GetLiquidDelegation(ctx, records[0].GetModuleAddress(), addrVal1)
				require.True(t, found, "delegation not found from tokenize share module account after redeem partial amount")

				records = app.StakingKeeper.GetAllTokenizeShareRecords(ctx)
				require.Len(t, records, 1)
			} else {
				_, found = app.StakingKeeper.GetLiquidDelegation(ctx, records[0].GetModuleAddress(), addrVal1)
				require.False(t, found, "delegation found from tokenize share module account after redeem full amount")

				records = app.StakingKeeper.GetAllTokenizeShareRecords(ctx)
				require.Len(t, records, 0)
			}
		})
	}
}

func TestTransferTokenizeShareRecord(t *testing.T) {
	_, app, ctx := createTestInput(t)

	addrs := simapp.AddTestAddrs(app, ctx, 3, app.StakingKeeper.TokensFromConsensusPower(ctx, 10000))
	addrAcc1, addrAcc2, valAcc := addrs[0], addrs[1], addrs[2]
	addrVal := sdk.ValAddress(valAcc)

	pubKeys := simapp.CreateTestPubKeys(1)
	pk := pubKeys[0]

	val := teststaking.NewValidator(t, addrVal, pk)
	app.StakingKeeper.SetValidator(ctx, val)
	app.StakingKeeper.SetValidatorByPowerIndex(ctx, val)

	// apply TM updates
	applyValidatorSetUpdates(t, ctx, app.StakingKeeper, -1)

	msgServer := keeper.NewMsgServerImpl(app.StakingKeeper)

	err := app.StakingKeeper.AddTokenizeShareRecord(ctx, types.TokenizeShareRecord{
		Id:            1,
		Owner:         addrAcc1.String(),
		ModuleAccount: "module_account",
		Validator:     val.String(),
	})
	require.NoError(t, err)

	_, err = msgServer.TransferTokenizeShareRecord(sdk.WrapSDKContext(ctx), &types.MsgTransferTokenizeShareRecord{
		TokenizeShareRecordId: 1,
		Sender:                addrAcc1.String(),
		NewOwner:              addrAcc2.String(),
	})
	require.NoError(t, err)

	record, err := app.StakingKeeper.GetTokenizeShareRecord(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, record.Owner, addrAcc2.String())

	records := app.StakingKeeper.GetTokenizeShareRecordsByOwner(ctx, addrAcc1)
	require.Len(t, records, 0)
	records = app.StakingKeeper.GetTokenizeShareRecordsByOwner(ctx, addrAcc2)
	require.Len(t, records, 1)
}

func TestValidatorBond(t *testing.T) {
	_, app, ctx := createTestInput(t)

	testCases := []struct {
		name                 string
		createValidator      bool
		createDelegation     bool
		alreadyValidatorBond bool
		delegatorIsLSTP      bool
		expectedErr          error
	}{
		{
			name:                 "successful validator bond",
			createValidator:      true,
			createDelegation:     true,
			alreadyValidatorBond: false,
			delegatorIsLSTP:      false,
		},
		{
			name:                 "successful with existing validator bond",
			createValidator:      true,
			createDelegation:     true,
			alreadyValidatorBond: true,
			delegatorIsLSTP:      false,
		},
		{
			name:                 "validator does not not exist",
			createValidator:      false,
			createDelegation:     false,
			alreadyValidatorBond: false,
			delegatorIsLSTP:      false,
			expectedErr:          sdkstaking.ErrNoValidatorFound,
		},
		{
			name:                 "delegation not exist case",
			createValidator:      true,
			createDelegation:     false,
			alreadyValidatorBond: false,
			delegatorIsLSTP:      false,
			expectedErr:          sdkstaking.ErrNoDelegation,
		},
		{
			name:                 "delegator is a liquid staking provider",
			createValidator:      true,
			createDelegation:     true,
			alreadyValidatorBond: false,
			delegatorIsLSTP:      true,
			expectedErr:          types.ErrValidatorBondNotAllowedFromModuleAccount,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, app, ctx = createTestInput(t)

			pubKeys := simapp.CreateTestPubKeys(2)
			validatorPubKey := pubKeys[0]
			delegatorPubKey := pubKeys[1]

			delegatorAddress := sdk.AccAddress(delegatorPubKey.Address())
			validatorAddress := sdk.ValAddress(validatorPubKey.Address())
			icaAccountAddress := createICAAccount(app, ctx, "ica-module-account")

			// Set the delegator address to either be a user account or an ICA account depending on the test case
			if tc.delegatorIsLSTP {
				delegatorAddress = icaAccountAddress
			}

			// Fund the delegator
			delegationAmount := app.StakingKeeper.TokensFromConsensusPower(ctx, 20)
			coins := sdk.NewCoins(sdk.NewCoin(app.StakingKeeper.BondDenom(ctx), delegationAmount))

			err := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins)
			require.NoError(t, err, "no error expected when minting")

			err = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, delegatorAddress, coins)
			require.NoError(t, err, "no error expected when funding account")

			// Create Validator and delegation
			if tc.createValidator {
				validator := teststaking.NewValidator(t, validatorAddress, validatorPubKey)
				validator.Status = sdkstaking.Bonded
				app.StakingKeeper.SetValidator(ctx, validator)
				app.StakingKeeper.SetValidatorByPowerIndex(ctx, validator)
				err = app.StakingKeeper.SetValidatorByConsAddr(ctx, validator)
				require.NoError(t, err)

				// Optionally create the delegation, depending on the test case
				if tc.createDelegation {
					_, err = app.StakingKeeper.Delegate(ctx, delegatorAddress, delegationAmount, sdkstaking.Unbonded, validator, true)
					require.NoError(t, err, "no error expected when delegating")

					// Optionally, convert the delegation into a validator bond
					if tc.alreadyValidatorBond {
						delegation, found := app.StakingKeeper.GetLiquidDelegation(ctx, delegatorAddress, validatorAddress)
						require.True(t, found, "delegation should have been found")

						delegation.ValidatorBond = true
						app.StakingKeeper.SetDelegation(ctx, delegation)
					}
				}
			}

			// Call ValidatorBond
			msgServer := keeper.NewMsgServerImpl(app.StakingKeeper)
			_, err = msgServer.ValidatorBond(sdk.WrapSDKContext(ctx), &types.MsgValidatorBond{
				DelegatorAddress: delegatorAddress.String(),
				ValidatorAddress: validatorAddress.String(),
			})

			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err, "no error expected from validator bond transaction")

				// check validator bond true
				delegation, found := app.StakingKeeper.GetLiquidDelegation(ctx, delegatorAddress, validatorAddress)
				require.True(t, found, "delegation should have been found after validator bond")
				require.True(t, delegation.ValidatorBond, "delegation should be marked as a validator bond")

				// check total validator bond shares
				validator, found := app.StakingKeeper.GetLiquidValidator(ctx, validatorAddress)
				require.True(t, found, "validator should have been found after validator bond")

				if tc.alreadyValidatorBond {
					require.True(t, validator.TotalValidatorBondShares.IsZero(), "validator total shares should still be zero")
				} else {
					require.Equal(t, delegation.Shares.String(), validator.TotalValidatorBondShares.String(),
						"validator total shares should have increased")
				}
			}
		})
	}
}

func TestEnableDisableTokenizeShares(t *testing.T) {
	_, app, ctx := createTestInput(t)
	msgServer := keeper.NewMsgServerImpl(app.StakingKeeper)

	// Create a delegator and validator
	stakeAmount := sdk.NewInt(1000)
	stakeToken := sdk.NewCoin(app.StakingKeeper.BondDenom(ctx), stakeAmount)

	addresses := simapp.AddTestAddrs(app, ctx, 2, stakeAmount)
	delegatorAddress := addresses[0]

	pubKeys := simapp.CreateTestPubKeys(1)
	validatorAddress := sdk.ValAddress(addresses[1])
	validator := teststaking.NewValidator(t, validatorAddress, pubKeys[0])

	validator.DelegatorShares = sdk.NewDec(1_000_000)
	validator.Tokens = sdk.NewInt(1_000_000)
	app.StakingKeeper.SetValidator(ctx, validator)

	// Fix block time and set unbonding period to 1 day
	blockTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx = ctx.WithBlockTime(blockTime)

	unbondingPeriod := time.Hour * 24
	params := app.StakingKeeper.GetParams(ctx)
	params.UnbondingTime = unbondingPeriod
	app.StakingKeeper.SetParams(ctx, params)

	// Build test messages (some of which will be reused)
	delegateMsg := types.MsgDelegate{
		DelegatorAddress: delegatorAddress.String(),
		ValidatorAddress: validatorAddress.String(),
		Amount:           stakeToken,
	}
	tokenizeMsg := types.MsgTokenizeShares{
		DelegatorAddress:    delegatorAddress.String(),
		ValidatorAddress:    validatorAddress.String(),
		Amount:              stakeToken,
		TokenizedShareOwner: delegatorAddress.String(),
	}
	redeemMsg := types.MsgRedeemTokensforShares{
		DelegatorAddress: delegatorAddress.String(),
	}
	disableMsg := types.MsgDisableTokenizeShares{
		DelegatorAddress: delegatorAddress.String(),
	}
	enableMsg := types.MsgEnableTokenizeShares{
		DelegatorAddress: delegatorAddress.String(),
	}

	// Delegate normally
	_, err := msgServer.Delegate(sdk.WrapSDKContext(ctx), &delegateMsg)
	require.NoError(t, err, "no error expected when delegating")

	// Tokenize shares - it should succeed
	_, err = msgServer.TokenizeShares(sdk.WrapSDKContext(ctx), &tokenizeMsg)
	require.NoError(t, err, "no error expected when tokenizing shares for the first time")

	liquidToken := app.BankKeeper.GetBalance(ctx, delegatorAddress, validatorAddress.String()+"/1")
	require.Equal(t, stakeAmount.Int64(), liquidToken.Amount.Int64(), "user received token after tokenizing share")

	// Redeem to remove all tokenized shares
	redeemMsg.Amount = liquidToken
	_, err = msgServer.RedeemTokens(sdk.WrapSDKContext(ctx), &redeemMsg)
	require.NoError(t, err, "no error expected when redeeming")

	// Attempt to enable tokenizing shares when there is no lock in place, it should error
	_, err = msgServer.EnableTokenizeShares(sdk.WrapSDKContext(ctx), &enableMsg)
	require.ErrorIs(t, err, types.ErrTokenizeSharesAlreadyEnabledForAccount)

	// Attempt to disable when no lock is in place, it should succeed
	_, err = msgServer.DisableTokenizeShares(sdk.WrapSDKContext(ctx), &disableMsg)
	require.NoError(t, err, "no error expected when disabling tokenization")

	// Disabling again while the lock is already in place, should error
	_, err = msgServer.DisableTokenizeShares(sdk.WrapSDKContext(ctx), &disableMsg)
	require.ErrorIs(t, err, types.ErrTokenizeSharesAlreadyDisabledForAccount)

	// Attempt to tokenize, it should fail since tokenization is disabled
	_, err = msgServer.TokenizeShares(sdk.WrapSDKContext(ctx), &tokenizeMsg)
	require.ErrorIs(t, err, types.ErrTokenizeSharesDisabledForAccount)

	// Now enable tokenization
	_, err = msgServer.EnableTokenizeShares(sdk.WrapSDKContext(ctx), &enableMsg)
	require.NoError(t, err, "no error expected when enabling tokenization")

	// Attempt to tokenize again, it should still fail since the unboning period has
	// not passed and the lock is still active
	_, err = msgServer.TokenizeShares(sdk.WrapSDKContext(ctx), &tokenizeMsg)
	require.ErrorIs(t, err, types.ErrTokenizeSharesDisabledForAccount)
	require.ErrorContains(t, err, fmt.Sprintf("tokenization will be allowed at %s",
		blockTime.Add(unbondingPeriod)))

	// Increment the block time by the unbonding period and remove the expired locks
	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(unbondingPeriod))
	app.StakingKeeper.RemoveExpiredTokenizeShareLocks(ctx, ctx.BlockTime())

	// Attempt to tokenize again, it should succeed this time since the lock has expired
	_, err = msgServer.TokenizeShares(sdk.WrapSDKContext(ctx), &tokenizeMsg)
	require.NoError(t, err, "no error expected when tokenizing after lock has expired")
}

func TestUnbondValidator(t *testing.T) {
	_, app, ctx := createTestInput(t)
	addrs := simapp.AddTestAddrs(app, ctx, 2, app.StakingKeeper.TokensFromConsensusPower(ctx, 10000))
	addrAcc1 := addrs[0]
	addrVal1 := sdk.ValAddress(addrAcc1)

	pubKeys := simapp.CreateTestPubKeys(1)
	pk1 := pubKeys[0]

	// Create Validators and Delegation
	val1 := teststaking.NewValidator(t, addrVal1, pk1)
	val1.Status = sdkstaking.Bonded
	app.StakingKeeper.SetValidator(ctx, val1)
	app.StakingKeeper.SetValidatorByPowerIndex(ctx, val1)
	err := app.StakingKeeper.SetValidatorByConsAddr(ctx, val1)
	require.NoError(t, err)

	// try unbonding not available validator
	msgServer := keeper.NewMsgServerImpl(app.StakingKeeper)
	_, err = msgServer.UnbondValidator(sdk.WrapSDKContext(ctx), &types.MsgUnbondValidator{
		ValidatorAddress: sdk.ValAddress(addrs[1]).String(),
	})
	require.Error(t, err)

	// unbond validator
	_, err = msgServer.UnbondValidator(sdk.WrapSDKContext(ctx), &types.MsgUnbondValidator{
		ValidatorAddress: addrVal1.String(),
	})
	require.NoError(t, err)

	// check if validator is jailed
	validator, found := app.StakingKeeper.GetValidator(ctx, addrVal1)
	require.True(t, found)
	require.True(t, validator.Jailed)
}
