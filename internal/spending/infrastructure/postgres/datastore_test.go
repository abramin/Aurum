package postgres_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/postgres"
)

// DataStoreSuite tests DataStore transaction behavior against a real Postgres instance.
//
// Justification: Transaction commit/rollback semantics, panic handling, and concurrent
// access patterns require real database behavior that cannot be mocked accurately.
type DataStoreSuite struct {
	suite.Suite
	ctx       context.Context
	dataStore *postgres.DataStore
}

func TestDataStoreSuite(t *testing.T) {
	suite.Run(t, new(DataStoreSuite))
}

func (s *DataStoreSuite) SetupTest() {
	s.ctx = context.Background()
	s.Require().NoError(truncateTables(s.ctx, getTestPool()))
	s.dataStore = postgres.NewDataStore(getTestPool())
}

func (s *DataStoreSuite) newCardAccount(tenantID string, limitAmount int64) *domain.CardAccount {
	limit := types.NewMoney(decimal.NewFromInt(limitAmount), types.CurrencyEUR)
	account, err := domain.NewCardAccount(types.TenantID(tenantID), limit, time.Now().UTC())
	s.Require().NoError(err)
	return account
}

func (s *DataStoreSuite) TestTransactionBehavior() {
	s.Run("successful callback commits all changes", func() {
		account := s.newCardAccount("tenant-commit", 1000)

		err := s.dataStore.Atomic(s.ctx, func(repos domain.Repositories) error {
			return repos.CardAccounts().Save(s.ctx, account)
		})
		s.Require().NoError(err)

		// Verify data persisted
		found, err := s.dataStore.CardAccounts().FindByID(s.ctx, account.TenantID(), account.ID())
		s.Require().NoError(err)
		s.Equal(account.ID(), found.ID())
	})

	s.Run("error in callback rolls back all changes", func() {
		account := s.newCardAccount("tenant-rollback", 1000)
		testErr := errors.New("simulated failure")

		err := s.dataStore.Atomic(s.ctx, func(repos domain.Repositories) error {
			if err := repos.CardAccounts().Save(s.ctx, account); err != nil {
				return err
			}
			return testErr // Return error after save
		})
		s.ErrorIs(err, testErr)

		// Verify data was NOT persisted
		_, err = s.dataStore.CardAccounts().FindByID(s.ctx, account.TenantID(), account.ID())
		s.ErrorIs(err, domain.ErrCardAccountNotFound)
	})

	s.Run("panic in callback rolls back and re-panics", func() {
		account := s.newCardAccount("tenant-panic", 1000)

		s.Panics(func() {
			_ = s.dataStore.Atomic(s.ctx, func(repos domain.Repositories) error {
				if err := repos.CardAccounts().Save(s.ctx, account); err != nil {
					return err
				}
				panic("simulated panic")
			})
		})

		// Verify data was NOT persisted
		_, err := s.dataStore.CardAccounts().FindByID(s.ctx, account.TenantID(), account.ID())
		s.ErrorIs(err, domain.ErrCardAccountNotFound)
	})

	s.Run("multiple writes in single transaction are atomic", func() {
		account := s.newCardAccount("tenant-multi-write", 1000)

		err := s.dataStore.Atomic(s.ctx, func(repos domain.Repositories) error {
			// Save initial account
			if err := repos.CardAccounts().Save(s.ctx, account); err != nil {
				return err
			}

			// Authorize and save again
			amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
			if err := account.AuthorizeAmount(amount, time.Now().UTC()); err != nil {
				return err
			}
			return repos.CardAccounts().Save(s.ctx, account)
		})
		s.Require().NoError(err)

		// Verify final state
		found, err := s.dataStore.CardAccounts().FindByID(s.ctx, account.TenantID(), account.ID())
		s.Require().NoError(err)
		s.Equal(2, found.Version())
		expected := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		s.True(found.RollingSpend().Equal(expected))
	})
}

func (s *DataStoreSuite) TestConcurrentSpendingLimitEnforcement() {
	s.Run("concurrent authorizations respect spending limit", func() {
		// Setup: Create account with 1000 EUR limit
		account := s.newCardAccount("tenant-concurrent", 1000)
		err := s.dataStore.Atomic(s.ctx, func(repos domain.Repositories) error {
			return repos.CardAccounts().Save(s.ctx, account)
		})
		s.Require().NoError(err)

		// 20 goroutines each try to authorize 100 EUR
		// Only 10 should succeed (1000 / 100 = 10)
		const goroutines = 20
		const authAmount = 100

		var wg sync.WaitGroup
		var successCount atomic.Int32
		var failCount atomic.Int32

		for range goroutines {
			wg.Go(func() {

				err := s.dataStore.Atomic(s.ctx, func(repos domain.Repositories) error {
					// Load fresh copy
					acc, err := repos.CardAccounts().FindByTenantID(s.ctx, "tenant-concurrent")
					if err != nil {
						return err
					}

					// Try to authorize
					amount := types.NewMoney(decimal.NewFromInt(authAmount), types.CurrencyEUR)
					if err := acc.AuthorizeAmount(amount, time.Now().UTC()); err != nil {
						return err
					}

					return repos.CardAccounts().Save(s.ctx, acc)
				})

				if err == nil {
					successCount.Add(1)
				} else {
					failCount.Add(1)
				}
			})
		}

		wg.Wait()

		// Verify final state
		final, err := s.dataStore.CardAccounts().FindByTenantID(s.ctx, "tenant-concurrent")
		s.Require().NoError(err)

		// Rolling spend should never exceed limit
		s.True(
			final.RollingSpend().LessThanOrEqual(final.SpendingLimit()),
			"rolling spend %s should not exceed limit %s",
			final.RollingSpend().String(),
			final.SpendingLimit().String(),
		)

		// Some should have succeeded, some failed
		s.Greater(successCount.Load(), int32(0), "at least one authorization should succeed")
		s.Greater(failCount.Load(), int32(0), "some authorizations should fail due to limit or conflicts")

		// Total successful * authAmount should equal rolling spend
		expectedSpend := types.NewMoney(decimal.NewFromInt(int64(successCount.Load())*authAmount), types.CurrencyEUR)
		s.True(
			final.RollingSpend().Equal(expectedSpend),
			"rolling spend %s should equal %d successes * %d EUR",
			final.RollingSpend().String(),
			successCount.Load(),
			authAmount,
		)
	})
}

func (s *DataStoreSuite) TestRepositoryAccess() {
	s.Run("all repositories are accessible within transaction", func() {
		account := s.newCardAccount("tenant-repos", 1000)

		err := s.dataStore.Atomic(s.ctx, func(repos domain.Repositories) error {
			// Access all repositories
			s.NotNil(repos.CardAccounts())
			s.NotNil(repos.Authorizations())
			s.NotNil(repos.IdempotencyStore())
			s.NotNil(repos.Outbox())

			return repos.CardAccounts().Save(s.ctx, account)
		})
		s.Require().NoError(err)
	})
}
