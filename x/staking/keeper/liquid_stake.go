package keeper

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdkstaking "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/iqlusioninc/liquidity-staking-module/x/staking/types"
)

// SetTotalLiquidStakedTokens stores the total outstanding tokens owned by a liquid staking provider
func (k Keeper) SetTotalLiquidStakedTokens(ctx sdk.Context, tokens sdk.Int) {
	store := ctx.KVStore(k.storeKey)

	tokensBz, err := tokens.Marshal()
	if err != nil {
		panic(err)
	}

	store.Set(types.TotalLiquidStakedTokensKey, tokensBz)
}

// GetTotalLiquidStakedTokens returns the total outstanding tokens owned by a liquid staking provider
// Returns zero if the total liquid stake amount has not been initialized
func (k Keeper) GetTotalLiquidStakedTokens(ctx sdk.Context) sdk.Int {
	store := ctx.KVStore(k.storeKey)
	tokensBz := store.Get(types.TotalLiquidStakedTokensKey)

	if tokensBz == nil {
		return sdk.ZeroInt()
	}

	var tokens sdk.Int
	if err := tokens.Unmarshal(tokensBz); err != nil {
		panic(err)
	}

	return tokens
}

// Check if an account is a owned by a liquid staking provider
// This is determined by checking if the account is a 32-length module account
func (k Keeper) AccountIsLiquidStakingProvider(ctx sdk.Context, address sdk.AccAddress) bool {
	account := k.authKeeper.GetAccount(ctx, address)
	_, isModuleAccount := account.(*authtypes.ModuleAccount)
	return isModuleAccount && len(address) == 32
}

// CheckExceedsGlobalLiquidStakingCap checks if a liquid delegation would cause the
// global liquid staking cap to be exceeded
// A liquid delegation is defined as either tokenized shares, or a delegation from an ICA Account
// The total stake is determined by the balance of the bonded pool
// If the delegation is for a tokenized share, the tokens are already included in the bonded pool
// If the delegation is from an ICA account, we need to add the tokens to the current bonded pool
// balance to get the total staked
// Returns true if the cap is exceeded
func (k Keeper) CheckExceedsGlobalLiquidStakingCap(ctx sdk.Context, tokens sdk.Int, tokenizingShares bool) bool {
	liquidStakingCap := k.GlobalLiquidStakingCap(ctx)
	liquidStakedAmount := k.GetTotalLiquidStakedTokens(ctx)

	// Determine the total stake from the balance of the bonded pool
	// If this is not a tokenized delegation, we need to add the tokens to the pool balance since
	// they would not have been counted yet
	// If this is for a tokenized delegation, the tokens are already included in the pool balance
	totalStakedAmount := k.TotalBondedTokens(ctx)
	if !tokenizingShares {
		totalStakedAmount = totalStakedAmount.Add(tokens)
	}

	// Calculate the percentage of stake that is liquid
	updatedLiquidStaked := liquidStakedAmount.Add(tokens).ToDec()
	liquidStakePercent := updatedLiquidStaked.Quo(totalStakedAmount.ToDec())

	return liquidStakePercent.GT(liquidStakingCap)
}

// CheckExceedsValidatorBondCap checks if a liquid delegation to a validator would cause
// the liquid shares to exceed the validator bond factor
// A liquid delegation is defined as either tokenized shares, or a delegation from an ICA Account
// Returns true if the cap is exceeded
func (k Keeper) CheckExceedsValidatorBondCap(ctx sdk.Context, validator types.Validator, shares sdk.Dec) bool {
	validatorBondFactor := k.ValidatorBondFactor(ctx)
	if validatorBondFactor.Equal(sdk.NewDec(-1)) {
		return false
	}
	maxValLiquidShares := validator.TotalValidatorBondShares.Mul(validatorBondFactor)
	return validator.TotalLiquidShares.Add(shares).GT(maxValLiquidShares)
}

// CheckExceedsValidatorLiquidStakingCap checks if a liquid delegation could cause the
// total liuquid shares to exceed the liquid staking cap
// A liquid delegation is defined as either tokenized shares, or a delegation from an ICA Account
// Returns true if the cap is exceeded
func (k Keeper) CheckExceedsValidatorLiquidStakingCap(ctx sdk.Context, validator types.Validator, shares sdk.Dec) bool {
	updatedLiquidShares := validator.TotalLiquidShares.Add(shares)
	updatedTotalShares := validator.DelegatorShares.Add(shares)

	liquidStakePercent := updatedLiquidShares.Quo(updatedTotalShares)
	liquidStakingCap := k.ValidatorLiquidStakingCap(ctx)

	return liquidStakePercent.GT(liquidStakingCap)
}

// SafelyIncreaseTotalLiquidStakedTokens increments the total liquid staked tokens
// if the global cap is not surpassed by this delegation
func (k Keeper) SafelyIncreaseTotalLiquidStakedTokens(ctx sdk.Context, amount sdk.Int, tokenizingShares bool) error {
	if k.CheckExceedsGlobalLiquidStakingCap(ctx, amount, tokenizingShares) {
		return types.ErrGlobalLiquidStakingCapExceeded
	}

	k.SetTotalLiquidStakedTokens(ctx, k.GetTotalLiquidStakedTokens(ctx).Add(amount))
	return nil
}

// DecreaseTotalLiquidStakedTokens decrements the total liquid staked tokens
func (k Keeper) DecreaseTotalLiquidStakedTokens(ctx sdk.Context, amount sdk.Int) {
	k.SetTotalLiquidStakedTokens(ctx, k.GetTotalLiquidStakedTokens(ctx).Sub(amount))
}

// SafelyIncreaseValidatorTotalLiquidShares increments the total liquid shares on a validator, if:
// the validator bond factor and validator liquid staking cap will not be exceeded by this delegation
func (k Keeper) SafelyIncreaseValidatorTotalLiquidShares(ctx sdk.Context, validator types.Validator, shares sdk.Dec) error {
	// Confirm the validator bond factor and validator liquid staking cap will be not exceeded
	if k.CheckExceedsValidatorBondCap(ctx, validator, shares) {
		return types.ErrInsufficientValidatorBondShares
	}
	if k.CheckExceedsValidatorLiquidStakingCap(ctx, validator, shares) {
		return types.ErrValidatorLiquidStakingCapExceeded
	}

	// Increment the validator's total liquid shares
	validator.TotalLiquidShares = validator.TotalLiquidShares.Add(shares)
	k.SetValidator(ctx, validator)

	return nil
}

// DecreaseValidatorTotalLiquidShares decrements the total liquid shares on a validator
func (k Keeper) DecreaseValidatorTotalLiquidShares(ctx sdk.Context, validator types.Validator, shares sdk.Dec) {
	validator.TotalLiquidShares = validator.TotalLiquidShares.Sub(shares)
	k.SetValidator(ctx, validator)
}

// SafelyDecreaseValidatorBond decrements the total validator's self bond
// so long as it will not cause the current delegations to exceed the threshold
// set by validator bond factor
func (k Keeper) SafelyDecreaseValidatorBond(ctx sdk.Context, validator types.Validator, shares sdk.Dec) error {
	// Check if the decreased self bond will cause the validator bond threshold to be exceeded
	validatorBondFactor := k.ValidatorBondFactor(ctx)
	maxValTotalShare := validator.TotalValidatorBondShares.Sub(shares).Mul(validatorBondFactor)
	if validator.TotalLiquidShares.GT(maxValTotalShare) {
		return types.ErrInsufficientValidatorBondShares
	}

	// Decrement the validator's total self bond
	validator.TotalValidatorBondShares = validator.TotalValidatorBondShares.Sub(shares)
	k.SetValidator(ctx, validator)

	return nil
}

// Adds a lock that prevents tokenizing shares for an account
// The tokenize share lock store is implemented by keying on the account address
// and storing a timestamp as the value. The timestamp is empty when the lock is
// set and gets populated with the unlock completion time once the unlock has started
func (k Keeper) AddTokenizeSharesLock(ctx sdk.Context, address sdk.AccAddress) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetTokenizeSharesLockKey(address)
	store.Set(key, sdk.FormatTimeBytes(time.Time{}))
}

// Removes the tokenize share lock for an account to enable tokenizing shares
func (k Keeper) RemoveTokenizeSharesLock(ctx sdk.Context, address sdk.AccAddress) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetTokenizeSharesLockKey(address)
	store.Delete(key)
}

// Updates the timestamp associated with a lock to the time at which the lock expires
func (k Keeper) SetTokenizeSharesUnlockTime(ctx sdk.Context, address sdk.AccAddress, completionTime time.Time) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetTokenizeSharesLockKey(address)
	store.Set(key, sdk.FormatTimeBytes(completionTime))
}

// Checks if there is currently a tokenize share lock for a given account
// Returns the status indicating whether the account is locked, unlocked,
// or as a lock expiring. If the lock is expiring, the expiration time is returned
func (k Keeper) GetTokenizeSharesLock(ctx sdk.Context, address sdk.AccAddress) (status types.TokenizeShareLockStatus, unlockTime time.Time) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetTokenizeSharesLockKey(address)
	bz := store.Get(key)
	if len(bz) == 0 {
		return types.TokenizeShareLockStatus_UNLOCKED, time.Time{}
	}
	unlockTime, err := sdk.ParseTimeBytes(bz)
	if err != nil {
		panic(err)
	}
	if unlockTime.IsZero() {
		return types.TokenizeShareLockStatus_LOCKED, time.Time{}
	}
	return types.TokenizeShareLockStatus_LOCK_EXPIRING, unlockTime
}

// Stores a list of addresses pending tokenize share unlocking at the same time
func (k Keeper) SetPendingTokenizeShareAuthorizations(ctx sdk.Context, completionTime time.Time, authorizations types.PendingTokenizeShareAuthorizations) {
	store := ctx.KVStore(k.storeKey)
	timeKey := types.GetTokenizeShareAuthorizationTimeKey(completionTime)
	bz := k.cdc.MustMarshal(&authorizations)
	store.Set(timeKey, bz)
}

// Returns a list of addresses pending tokenize share unlocking at the same time
func (k Keeper) GetPendingTokenizeShareAuthorizations(ctx sdk.Context, completionTime time.Time) types.PendingTokenizeShareAuthorizations {
	store := ctx.KVStore(k.storeKey)

	timeKey := types.GetTokenizeShareAuthorizationTimeKey(completionTime)
	bz := store.Get(timeKey)

	authorizations := types.PendingTokenizeShareAuthorizations{Addresses: []string{}}
	if len(bz) == 0 {
		return authorizations
	}
	k.cdc.MustUnmarshal(bz, &authorizations)

	return authorizations
}

// Inserts the address into a queue where it will sit for 1 unbonding period
// before the tokenize share lock is removed
// Returns the completion time
func (k Keeper) QueueTokenizeSharesAuthorization(ctx sdk.Context, address sdk.AccAddress) time.Time {
	params := k.GetParams(ctx)
	completionTime := ctx.BlockTime().Add(params.UnbondingTime)

	// Append the address to the list of addresses that also unlock at this time
	authorizations := k.GetPendingTokenizeShareAuthorizations(ctx, completionTime)
	authorizations.Addresses = append(authorizations.Addresses, address.String())

	k.SetPendingTokenizeShareAuthorizations(ctx, completionTime, authorizations)
	k.SetTokenizeSharesUnlockTime(ctx, address, completionTime)

	return completionTime
}

// Unlocks all queued tokenize share authorizations that have matured
// (i.e. have waited the full unbonding period)
func (k Keeper) RemoveExpiredTokenizeShareLocks(ctx sdk.Context, blockTime time.Time) (unlockedAddresses []string) {
	store := ctx.KVStore(k.storeKey)

	// iterators all time slices from time 0 until the current block time
	prefixEnd := sdk.InclusiveEndBytes(types.GetTokenizeShareAuthorizationTimeKey(blockTime))
	iterator := store.Iterator(types.TokenizeSharesUnlockQueueKey, prefixEnd)
	defer iterator.Close()

	unlockedAddresses = []string{}
	for ; iterator.Valid(); iterator.Next() {
		authorizations := types.PendingTokenizeShareAuthorizations{}
		k.cdc.MustUnmarshal(iterator.Value(), &authorizations)

		for _, addressString := range authorizations.Addresses {
			k.RemoveTokenizeSharesLock(ctx, sdk.MustAccAddressFromBech32(addressString))
			unlockedAddresses = append(unlockedAddresses, addressString)
		}
		store.Delete(iterator.Key())
	}

	return unlockedAddresses
}

// Calculates and sets the global liquid staked tokens and total liquid shares by validator
// The totals are determined by looping each delegation record and summing the stake
// if the delegator is a module account. Checking for a module account will capture
// ICA accounts, as well as tokenized delegationswhich are owned by module accounts
// under the hood
// This function must be called in the upgrade handler which onboards LSM, as
// well as any time the liquid staking cap is re-enabled
func (k Keeper) RefreshTotalLiquidStaked(ctx sdk.Context) error {
	// First reset each validator's liquid shares to 0
	for _, validator := range k.GetAllValidators(ctx) {
		validator.TotalLiquidShares = sdk.ZeroDec()
		k.SetValidator(ctx, validator)
	}

	// Sum up the total liquid tokens and increment each validator's total liquid shares
	totalLiquidStakedTokens := sdk.ZeroInt()
	for _, delegation := range k.GetAllDelegations(ctx) {
		delegatorAddress, err := sdk.AccAddressFromBech32(delegation.DelegatorAddress)
		if err != nil {
			return err
		}
		validatorAddress, err := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
		if err != nil {
			return err
		}

		validator, found := k.GetLiquidValidator(ctx, validatorAddress)
		if !found {
			return sdkstaking.ErrNoValidatorFound
		}

		// If the account is a liquid staking provider, increment the global number
		// of liquid staked tokens, and the total liquid shares on the validator
		if k.AccountIsLiquidStakingProvider(ctx, delegatorAddress) {
			liquidShares := delegation.Shares
			liquidTokens := validator.TokensFromShares(liquidShares).TruncateInt()

			validator.TotalLiquidShares = validator.TotalLiquidShares.Add(liquidShares)
			k.SetValidator(ctx, validator)

			totalLiquidStakedTokens = totalLiquidStakedTokens.Add(liquidTokens)
		}
	}

	k.SetTotalLiquidStakedTokens(ctx, totalLiquidStakedTokens)

	return nil
}
