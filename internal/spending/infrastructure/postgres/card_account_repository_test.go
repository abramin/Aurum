package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/postgres"
)

// CardAccountRepositorySuite tests CardAccountRepository behavior against a real Postgres instance.
//
// Justification: UPSERT with optimistic locking (version check in WHERE clause) requires
// real Postgres to verify row-level behavior and RowsAffected semantics.
type CardAccountRepositorySuite struct {
	suite.Suite
	ctx  context.Context
	repo *postgres.CardAccountRepository
}

func TestCardAccountRepositorySuite(t *testing.T) {
	suite.Run(t, new(CardAccountRepositorySuite))
}

func (s *CardAccountRepositorySuite) SetupTest() {
	s.ctx = context.Background()
	s.Require().NoError(truncateTables(s.ctx, getTestPool()))
	s.repo = postgres.NewCardAccountRepository(getTestPool())
}

func (s *CardAccountRepositorySuite) newCardAccount(tenantID string) *domain.CardAccount {
	limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
	account, err := domain.NewCardAccount(types.TenantID(tenantID), limit, time.Now().UTC())
	s.Require().NoError(err)
	return account
}

func (s *CardAccountRepositorySuite) TestPersistence() {
	s.Run("Save creates new record with version 1", func() {
		account := s.newCardAccount("tenant-new")

		err := s.repo.Save(s.ctx, account)

		s.Require().NoError(err)

		found, err := s.repo.FindByID(s.ctx, account.TenantID(), account.ID())
		s.Require().NoError(err)
		s.Equal(account.ID(), found.ID())
		s.Equal(1, found.Version())
	})

	s.Run("Save updates existing record and increments version", func() {
		account := s.newCardAccount("tenant-update")
		err := s.repo.Save(s.ctx, account)
		s.Require().NoError(err)

		// Modify and save again
		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		err = account.AuthorizeAmount(amount, time.Now().UTC())
		s.Require().NoError(err)
		s.Equal(2, account.Version())

		err = s.repo.Save(s.ctx, account)
		s.Require().NoError(err)

		found, err := s.repo.FindByID(s.ctx, account.TenantID(), account.ID())
		s.Require().NoError(err)
		s.Equal(2, found.Version())
		s.True(found.RollingSpend().Equal(amount))
	})

	s.Run("FindByID retrieves correct record", func() {
		account := s.newCardAccount("tenant-find-id")
		err := s.repo.Save(s.ctx, account)
		s.Require().NoError(err)

		found, err := s.repo.FindByID(s.ctx, account.TenantID(), account.ID())

		s.Require().NoError(err)
		s.Equal(account.ID(), found.ID())
		s.Equal(account.TenantID(), found.TenantID())
		s.True(account.SpendingLimit().Equal(found.SpendingLimit()))
	})

	s.Run("FindByTenantID retrieves card account", func() {
		account := s.newCardAccount("tenant-find-tenant")
		err := s.repo.Save(s.ctx, account)
		s.Require().NoError(err)

		found, err := s.repo.FindByTenantID(s.ctx, account.TenantID())

		s.Require().NoError(err)
		s.Equal(account.ID(), found.ID())
	})

	s.Run("FindByID returns ErrCardAccountNotFound for missing", func() {
		missingID := domain.NewCardAccountID()

		_, err := s.repo.FindByID(s.ctx, "tenant-missing", missingID)

		s.ErrorIs(err, domain.ErrCardAccountNotFound)
	})

	s.Run("FindByTenantID returns ErrCardAccountNotFound for missing tenant", func() {
		_, err := s.repo.FindByTenantID(s.ctx, "nonexistent-tenant")

		s.ErrorIs(err, domain.ErrCardAccountNotFound)
	})
}

func (s *CardAccountRepositorySuite) TestOptimisticLocking() {
	s.Run("Save with stale version returns ErrOptimisticLock", func() {
		// Create and save initial account
		account := s.newCardAccount("tenant-lock")
		err := s.repo.Save(s.ctx, account)
		s.Require().NoError(err)

		// Load a second copy (simulating another process)
		staleCopy, err := s.repo.FindByID(s.ctx, account.TenantID(), account.ID())
		s.Require().NoError(err)

		// Update original and save (version 1 -> 2)
		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		err = account.AuthorizeAmount(amount, time.Now().UTC())
		s.Require().NoError(err)
		err = s.repo.Save(s.ctx, account)
		s.Require().NoError(err)

		// Try to save stale copy (still version 1, but DB has version 2)
		err = staleCopy.AuthorizeAmount(amount, time.Now().UTC())
		s.Require().NoError(err)
		s.Equal(2, staleCopy.Version(), "stale copy incremented locally")

		err = s.repo.Save(s.ctx, staleCopy)

		s.ErrorIs(err, domain.ErrOptimisticLock, "should detect version conflict")
	})

	s.Run("consecutive saves with correct versions succeed", func() {
		account := s.newCardAccount("tenant-sequential")
		amount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyEUR)

		for i := range 5 {
			if i > 0 {
				err := account.AuthorizeAmount(amount, time.Now().UTC())
				s.Require().NoError(err)
			}
			err := s.repo.Save(s.ctx, account)
			s.Require().NoError(err, "save %d should succeed", i+1)
		}

		found, err := s.repo.FindByID(s.ctx, account.TenantID(), account.ID())
		s.Require().NoError(err)
		s.Equal(5, found.Version())
	})
}

func (s *CardAccountRepositorySuite) TestDataIntegrity() {
	s.Run("persists money amounts correctly", func() {
		limit := types.NewMoney(decimal.NewFromFloat(1234.5678), types.CurrencyEUR)
		account, err := domain.NewCardAccount("tenant-money", limit, time.Now().UTC())
		s.Require().NoError(err)

		spend := types.NewMoney(decimal.NewFromFloat(123.4567), types.CurrencyEUR)
		err = account.AuthorizeAmount(spend, time.Now().UTC())
		s.Require().NoError(err)

		err = s.repo.Save(s.ctx, account)
		s.Require().NoError(err)

		found, err := s.repo.FindByID(s.ctx, account.TenantID(), account.ID())
		s.Require().NoError(err)

		s.True(found.SpendingLimit().Equal(limit), "spending limit should be preserved")
		s.True(found.RollingSpend().Equal(spend), "rolling spend should be preserved")
	})

	s.Run("preserves timestamps", func() {
		now := time.Now().UTC().Truncate(time.Microsecond)
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		account, err := domain.NewCardAccount("tenant-time", limit, now)
		s.Require().NoError(err)

		err = s.repo.Save(s.ctx, account)
		s.Require().NoError(err)

		found, err := s.repo.FindByID(s.ctx, account.TenantID(), account.ID())
		s.Require().NoError(err)

		s.WithinDuration(now, found.CreatedAt(), time.Millisecond)
		s.WithinDuration(now, found.UpdatedAt(), time.Millisecond)
	})
}
