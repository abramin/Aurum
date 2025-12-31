package infrastructure

import (
	"context"
	"sync"

	"aurum/internal/spending/domain"

	vo "aurum/internal/common/value_objects"
)

// MemoryAuthorizationRepository is an in-memory implementation of AuthorizationRepository.
type MemoryAuthorizationRepository struct {
	mu      sync.RWMutex
	storage map[string]*domain.Authorization // key: tenantID:id
}

// NewMemoryAuthorizationRepository creates a new in-memory authorization repository.
func NewMemoryAuthorizationRepository() *MemoryAuthorizationRepository {
	return &MemoryAuthorizationRepository{
		storage: make(map[string]*domain.Authorization),
	}
}

func (r *MemoryAuthorizationRepository) key(tenantID vo.TenantID, id domain.AuthorizationID) string {
	return tenantID.String() + ":" + id.String()
}

func (r *MemoryAuthorizationRepository) Save(_ context.Context, auth *domain.Authorization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.storage[r.key(auth.TenantID(), auth.ID())] = auth
	return nil
}

func (r *MemoryAuthorizationRepository) FindByID(_ context.Context, tenantID vo.TenantID, id domain.AuthorizationID) (*domain.Authorization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.storage[r.key(tenantID, id)], nil
}

// MemoryCardAccountRepository is an in-memory implementation of CardAccountRepository.
// Note: byTenant index supports multiple card accounts per tenant. FindByTenantID
// returns the first one found (use FindByID for specific lookups).
type MemoryCardAccountRepository struct {
	mu       sync.RWMutex
	storage  map[string]*domain.CardAccount   // key: tenantID:id
	byTenant map[string][]*domain.CardAccount // key: tenantID string -> slice of accounts
}

// NewMemoryCardAccountRepository creates a new in-memory card account repository.
func NewMemoryCardAccountRepository() *MemoryCardAccountRepository {
	return &MemoryCardAccountRepository{
		storage:  make(map[string]*domain.CardAccount),
		byTenant: make(map[string][]*domain.CardAccount),
	}
}

func (r *MemoryCardAccountRepository) key(tenantID vo.TenantID, id domain.CardAccountID) string {
	return tenantID.String() + ":" + id.String()
}

func (r *MemoryCardAccountRepository) Save(_ context.Context, account *domain.CardAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.key(account.TenantID(), account.ID())
	tenantKey := account.TenantID().String()

	// Check if this account already exists (update case)
	_, exists := r.storage[key]
	r.storage[key] = account

	if !exists {
		// New account - add to tenant index
		r.byTenant[tenantKey] = append(r.byTenant[tenantKey], account)
	} else {
		// Update existing - find and replace in tenant index
		accounts := r.byTenant[tenantKey]
		for i, a := range accounts {
			if a.ID().String() == account.ID().String() {
				accounts[i] = account
				break
			}
		}
	}
	return nil
}

func (r *MemoryCardAccountRepository) FindByID(_ context.Context, tenantID vo.TenantID, id domain.CardAccountID) (*domain.CardAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.storage[r.key(tenantID, id)], nil
}

// FindByTenantID returns the first card account for the tenant, or nil if none exist.
// Use FindByID for specific account lookups when multiple accounts exist per tenant.
func (r *MemoryCardAccountRepository) FindByTenantID(_ context.Context, tenantID vo.TenantID) (*domain.CardAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	accounts := r.byTenant[tenantID.String()]
	if len(accounts) == 0 {
		return nil, nil
	}
	return accounts[0], nil
}

// MemoryIdempotencyStore is an in-memory implementation of IdempotencyStore.
type MemoryIdempotencyStore struct {
	mu      sync.Mutex
	storage map[string]*domain.IdempotencyEntry // key: tenantID:idempotencyKey
}

// NewMemoryIdempotencyStore creates a new in-memory idempotency store.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		storage: make(map[string]*domain.IdempotencyEntry),
	}
}

func (s *MemoryIdempotencyStore) key(tenantID vo.TenantID, idempotencyKey string) string {
	return tenantID.String() + ":" + idempotencyKey
}

func (s *MemoryIdempotencyStore) Get(_ context.Context, tenantID vo.TenantID, key string) (*domain.IdempotencyEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storage[s.key(tenantID, key)], nil
}

func (s *MemoryIdempotencyStore) Set(_ context.Context, entry *domain.IdempotencyEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storage[s.key(entry.TenantID, entry.IdempotencyKey)] = entry
	return nil
}

// SetIfAbsent atomically stores an entry if no entry exists for the key.
// Returns true and nil if the entry was created.
// Returns false and the existing entry if it already existed.
func (s *MemoryIdempotencyStore) SetIfAbsent(_ context.Context, entry *domain.IdempotencyEntry) (bool, *domain.IdempotencyEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.key(entry.TenantID, entry.IdempotencyKey)
	if existing, ok := s.storage[k]; ok {
		return false, existing, nil
	}
	s.storage[k] = entry
	return true, nil, nil
}

// MemoryUnitOfWork implements UnitOfWork for in-memory repositories.
type MemoryUnitOfWork struct {
	authRepo        *MemoryAuthorizationRepository
	cardAccountRepo *MemoryCardAccountRepository
}

// NewMemoryUnitOfWork creates a new in-memory unit of work.
func NewMemoryUnitOfWork(authRepo *MemoryAuthorizationRepository, cardAccountRepo *MemoryCardAccountRepository) *MemoryUnitOfWork {
	return &MemoryUnitOfWork{
		authRepo:        authRepo,
		cardAccountRepo: cardAccountRepo,
	}
}

// Begin starts a new transaction.
func (uow *MemoryUnitOfWork) Begin(_ context.Context) (domain.Transaction, error) {
	return &MemoryTransaction{
		authRepo:               uow.authRepo,
		cardAccountRepo:        uow.cardAccountRepo,
		stagedAuthorizations:   make(map[string]*domain.Authorization),
		stagedCardAccounts:     make(map[string]*domain.CardAccount),
	}, nil
}

// MemoryTransaction represents an in-memory transaction with staged changes.
type MemoryTransaction struct {
	authRepo               *MemoryAuthorizationRepository
	cardAccountRepo        *MemoryCardAccountRepository
	stagedAuthorizations   map[string]*domain.Authorization
	stagedCardAccounts     map[string]*domain.CardAccount
}

// AuthorizationRepo returns a transactional authorization repository.
func (tx *MemoryTransaction) AuthorizationRepo() domain.AuthorizationRepository {
	return &txAuthorizationRepo{tx: tx}
}

// CardAccountRepo returns a transactional card account repository.
func (tx *MemoryTransaction) CardAccountRepo() domain.CardAccountRepository {
	return &txCardAccountRepo{tx: tx}
}

// Commit atomically applies all staged changes.
func (tx *MemoryTransaction) Commit() error {
	// Lock both repositories during commit for atomicity
	tx.authRepo.mu.Lock()
	defer tx.authRepo.mu.Unlock()
	tx.cardAccountRepo.mu.Lock()
	defer tx.cardAccountRepo.mu.Unlock()

	// Apply all staged authorization changes
	for key, auth := range tx.stagedAuthorizations {
		tx.authRepo.storage[key] = auth
	}

	// Apply all staged card account changes
	for key, account := range tx.stagedCardAccounts {
		tenantKey := account.TenantID().String()
		_, exists := tx.cardAccountRepo.storage[key]
		tx.cardAccountRepo.storage[key] = account

		if !exists {
			tx.cardAccountRepo.byTenant[tenantKey] = append(tx.cardAccountRepo.byTenant[tenantKey], account)
		} else {
			accounts := tx.cardAccountRepo.byTenant[tenantKey]
			for i, a := range accounts {
				if a.ID().String() == account.ID().String() {
					accounts[i] = account
					break
				}
			}
		}
	}

	return nil
}

// Rollback discards all staged changes.
func (tx *MemoryTransaction) Rollback() error {
	tx.stagedAuthorizations = nil
	tx.stagedCardAccounts = nil
	return nil
}

// txAuthorizationRepo is a transactional authorization repository that stages changes.
type txAuthorizationRepo struct {
	tx *MemoryTransaction
}

func (r *txAuthorizationRepo) key(tenantID vo.TenantID, id domain.AuthorizationID) string {
	return tenantID.String() + ":" + id.String()
}

func (r *txAuthorizationRepo) Save(_ context.Context, auth *domain.Authorization) error {
	key := r.key(auth.TenantID(), auth.ID())
	r.tx.stagedAuthorizations[key] = auth
	return nil
}

func (r *txAuthorizationRepo) FindByID(_ context.Context, tenantID vo.TenantID, id domain.AuthorizationID) (*domain.Authorization, error) {
	key := r.key(tenantID, id)
	// Check staged changes first
	if auth, ok := r.tx.stagedAuthorizations[key]; ok {
		return auth, nil
	}
	// Fall back to real repository
	r.tx.authRepo.mu.RLock()
	defer r.tx.authRepo.mu.RUnlock()
	return r.tx.authRepo.storage[key], nil
}

// txCardAccountRepo is a transactional card account repository that stages changes.
type txCardAccountRepo struct {
	tx *MemoryTransaction
}

func (r *txCardAccountRepo) key(tenantID vo.TenantID, id domain.CardAccountID) string {
	return tenantID.String() + ":" + id.String()
}

func (r *txCardAccountRepo) Save(_ context.Context, account *domain.CardAccount) error {
	key := r.key(account.TenantID(), account.ID())
	r.tx.stagedCardAccounts[key] = account
	return nil
}

func (r *txCardAccountRepo) FindByID(_ context.Context, tenantID vo.TenantID, id domain.CardAccountID) (*domain.CardAccount, error) {
	key := r.key(tenantID, id)
	// Check staged changes first
	if account, ok := r.tx.stagedCardAccounts[key]; ok {
		return account, nil
	}
	// Fall back to real repository
	r.tx.cardAccountRepo.mu.RLock()
	defer r.tx.cardAccountRepo.mu.RUnlock()
	return r.tx.cardAccountRepo.storage[key], nil
}

func (r *txCardAccountRepo) FindByTenantID(_ context.Context, tenantID vo.TenantID) (*domain.CardAccount, error) {
	// Check staged changes first
	for _, account := range r.tx.stagedCardAccounts {
		if account.TenantID().String() == tenantID.String() {
			return account, nil
		}
	}
	// Fall back to real repository
	r.tx.cardAccountRepo.mu.RLock()
	defer r.tx.cardAccountRepo.mu.RUnlock()
	accounts := r.tx.cardAccountRepo.byTenant[tenantID.String()]
	if len(accounts) == 0 {
		return nil, nil
	}
	return accounts[0], nil
}
