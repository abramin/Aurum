package memory

import (
	"context"
	"sync"
	"time"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

// DataStore implements domain.AtomicExecutor and domain.Repositories for testing.
// It provides an in-memory implementation that supports the Atomic pattern.
// Concurrency: all access is guarded by a mutex.
type DataStore struct {
	mu                sync.RWMutex
	authorizations    map[string]*domain.Authorization
	cardAccounts      map[string]*domain.CardAccount
	idempotencyKeys   map[string]*domain.IdempotencyEntry
	outboxEntries     []*domain.OutboxEntry

	authorizationRepo *AuthorizationRepository
	cardAccountRepo   *CardAccountRepository
	idempotencyStore  *IdempotencyStore
	outboxRepo        *OutboxRepository
}

// NewDataStore creates a new in-memory DataStore.
func NewDataStore() *DataStore {
	ds := &DataStore{
		authorizations:  make(map[string]*domain.Authorization),
		cardAccounts:    make(map[string]*domain.CardAccount),
		idempotencyKeys: make(map[string]*domain.IdempotencyEntry),
		outboxEntries:   make([]*domain.OutboxEntry, 0),
	}

	ds.authorizationRepo = &AuthorizationRepository{store: ds}
	ds.cardAccountRepo = &CardAccountRepository{store: ds}
	ds.idempotencyStore = &IdempotencyStore{store: ds}
	ds.outboxRepo = &OutboxRepository{store: ds}

	return ds
}

// Authorizations returns the authorization repository.
func (ds *DataStore) Authorizations() domain.AuthorizationRepository {
	return ds.authorizationRepo
}

// CardAccounts returns the card account repository.
func (ds *DataStore) CardAccounts() domain.CardAccountRepository {
	return ds.cardAccountRepo
}

// IdempotencyStore returns the idempotency store.
func (ds *DataStore) IdempotencyStore() domain.IdempotencyStore {
	return ds.idempotencyStore
}

// Outbox returns the outbox repository.
func (ds *DataStore) Outbox() domain.OutboxRepository {
	return ds.outboxRepo
}

// Atomic executes the callback atomically.
// It locks the store, runs the callback against a transactional snapshot,
// and commits staged changes only if the callback succeeds.
// Concurrency: the store is locked for the duration of the callback.
func (ds *DataStore) Atomic(ctx context.Context, fn domain.AtomicCallback) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Create a transaction snapshot
	tx := &transactionalDataStore{
		parent:              ds,
		stagedAuthorizations: make(map[string]*domain.Authorization),
		stagedCardAccounts:   make(map[string]*domain.CardAccount),
		stagedIdempotency:    make(map[string]*domain.IdempotencyEntry),
		stagedOutbox:         make([]*domain.OutboxEntry, 0),
	}

	// Execute callback with transactional repos
	if err := fn(tx); err != nil {
		return err
	}

	// Commit: apply staged changes
	for k, v := range tx.stagedAuthorizations {
		ds.authorizations[k] = v
	}
	for k, v := range tx.stagedCardAccounts {
		ds.cardAccounts[k] = v
	}
	for k, v := range tx.stagedIdempotency {
		ds.idempotencyKeys[k] = v
	}
	ds.outboxEntries = append(ds.outboxEntries, tx.stagedOutbox...)

	return nil
}

// transactionalDataStore provides transaction isolation for memory operations.
type transactionalDataStore struct {
	parent               *DataStore
	stagedAuthorizations map[string]*domain.Authorization
	stagedCardAccounts   map[string]*domain.CardAccount
	stagedIdempotency    map[string]*domain.IdempotencyEntry
	stagedOutbox         []*domain.OutboxEntry
}

func (tx *transactionalDataStore) Authorizations() domain.AuthorizationRepository {
	return &txAuthorizationRepository{tx: tx}
}

func (tx *transactionalDataStore) CardAccounts() domain.CardAccountRepository {
	return &txCardAccountRepository{tx: tx}
}

func (tx *transactionalDataStore) IdempotencyStore() domain.IdempotencyStore {
	return &txIdempotencyStore{tx: tx}
}

func (tx *transactionalDataStore) Outbox() domain.OutboxRepository {
	return &txOutboxRepository{tx: tx}
}

// Transactional repository implementations

type txAuthorizationRepository struct {
	tx *transactionalDataStore
}

func (r *txAuthorizationRepository) Save(ctx context.Context, auth *domain.Authorization) error {
	key := auth.TenantID().String() + ":" + auth.ID().String()
	r.tx.stagedAuthorizations[key] = auth
	return nil
}

func (r *txAuthorizationRepository) FindByID(ctx context.Context, tenantID types.TenantID, id domain.AuthorizationID) (*domain.Authorization, error) {
	key := tenantID.String() + ":" + id.String()
	// Check staged first
	if auth, ok := r.tx.stagedAuthorizations[key]; ok {
		return auth, nil
	}
	// Then check parent
	if auth, ok := r.tx.parent.authorizations[key]; ok {
		return auth, nil
	}
	return nil, domain.ErrAuthorizationNotFound
}

type txCardAccountRepository struct {
	tx *transactionalDataStore
}

func (r *txCardAccountRepository) Save(ctx context.Context, account *domain.CardAccount) error {
	key := account.TenantID().String() + ":" + account.ID().String()
	r.tx.stagedCardAccounts[key] = account
	return nil
}

func (r *txCardAccountRepository) FindByID(ctx context.Context, tenantID types.TenantID, id domain.CardAccountID) (*domain.CardAccount, error) {
	key := tenantID.String() + ":" + id.String()
	// Check staged first
	if account, ok := r.tx.stagedCardAccounts[key]; ok {
		return account, nil
	}
	// Then check parent
	if account, ok := r.tx.parent.cardAccounts[key]; ok {
		return account, nil
	}
	return nil, domain.ErrCardAccountNotFound
}

func (r *txCardAccountRepository) FindByTenantID(ctx context.Context, tenantID types.TenantID) (*domain.CardAccount, error) {
	// Check staged first
	for _, account := range r.tx.stagedCardAccounts {
		if account.TenantID() == tenantID {
			return account, nil
		}
	}
	// Then check parent
	for _, account := range r.tx.parent.cardAccounts {
		if account.TenantID() == tenantID {
			return account, nil
		}
	}
	return nil, domain.ErrCardAccountNotFound
}

type txIdempotencyStore struct {
	tx *transactionalDataStore
}

func (s *txIdempotencyStore) Get(ctx context.Context, tenantID types.TenantID, key string) (*domain.IdempotencyEntry, error) {
	k := tenantID.String() + ":" + key
	// Check staged first
	if entry, ok := s.tx.stagedIdempotency[k]; ok {
		return entry, nil
	}
	// Then check parent
	if entry, ok := s.tx.parent.idempotencyKeys[k]; ok {
		return entry, nil
	}
	return nil, nil
}

func (s *txIdempotencyStore) Set(ctx context.Context, entry *domain.IdempotencyEntry) error {
	k := entry.TenantID.String() + ":" + entry.IdempotencyKey
	s.tx.stagedIdempotency[k] = entry
	return nil
}

func (s *txIdempotencyStore) SetIfAbsent(ctx context.Context, entry *domain.IdempotencyEntry) (bool, *domain.IdempotencyEntry, error) {
	existing, _ := s.Get(ctx, entry.TenantID, entry.IdempotencyKey)
	if existing != nil {
		return false, existing, nil
	}
	if err := s.Set(ctx, entry); err != nil {
		return false, nil, err
	}
	return true, entry, nil
}

type txOutboxRepository struct {
	tx *transactionalDataStore
}

func (r *txOutboxRepository) Append(ctx context.Context, entry *domain.OutboxEntry) error {
	r.tx.stagedOutbox = append(r.tx.stagedOutbox, entry)
	return nil
}

func (r *txOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
	var entries []*domain.OutboxEntry
	for _, entry := range r.tx.parent.outboxEntries {
		if entry.PublishedAt == nil {
			entries = append(entries, entry)
			if len(entries) >= limit {
				break
			}
		}
	}
	return entries, nil
}

func (r *txOutboxRepository) MarkPublished(ctx context.Context, ids []types.EventID) error {
	now := time.Now()
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id.String()] = true
	}
	for _, entry := range r.tx.parent.outboxEntries {
		if idSet[entry.ID.String()] {
			entry.PublishedAt = &now
		}
	}
	return nil
}

// Non-transactional repository implementations (for direct access)

// AuthorizationRepository provides non-transactional access to in-memory authorizations.
type AuthorizationRepository struct {
	store *DataStore
}

// Save stores an authorization in memory, overwriting any existing entry.
func (r *AuthorizationRepository) Save(ctx context.Context, auth *domain.Authorization) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	key := auth.TenantID().String() + ":" + auth.ID().String()
	r.store.authorizations[key] = auth
	return nil
}

// FindByID loads an authorization by tenant and ID from memory.
// Returns ErrAuthorizationNotFound when missing.
func (r *AuthorizationRepository) FindByID(ctx context.Context, tenantID types.TenantID, id domain.AuthorizationID) (*domain.Authorization, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	key := tenantID.String() + ":" + id.String()
	if auth, ok := r.store.authorizations[key]; ok {
		return auth, nil
	}
	return nil, domain.ErrAuthorizationNotFound
}

// CardAccountRepository provides non-transactional access to in-memory card accounts.
type CardAccountRepository struct {
	store *DataStore
}

// Save stores a card account in memory, overwriting any existing entry.
func (r *CardAccountRepository) Save(ctx context.Context, account *domain.CardAccount) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	key := account.TenantID().String() + ":" + account.ID().String()
	r.store.cardAccounts[key] = account
	return nil
}

// FindByID loads a card account by tenant and ID from memory.
// Returns ErrCardAccountNotFound when missing.
func (r *CardAccountRepository) FindByID(ctx context.Context, tenantID types.TenantID, id domain.CardAccountID) (*domain.CardAccount, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	key := tenantID.String() + ":" + id.String()
	if account, ok := r.store.cardAccounts[key]; ok {
		return account, nil
	}
	return nil, domain.ErrCardAccountNotFound
}

// FindByTenantID scans in-memory accounts for a matching tenant.
// Returns ErrCardAccountNotFound when missing.
func (r *CardAccountRepository) FindByTenantID(ctx context.Context, tenantID types.TenantID) (*domain.CardAccount, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	for _, account := range r.store.cardAccounts {
		if account.TenantID() == tenantID {
			return account, nil
		}
	}
	return nil, domain.ErrCardAccountNotFound
}

// IdempotencyStore provides non-transactional access to in-memory idempotency records.
type IdempotencyStore struct {
	store *DataStore
}

// Get retrieves an idempotency entry by tenant and key.
// Returns (nil, nil) when no entry exists.
func (s *IdempotencyStore) Get(ctx context.Context, tenantID types.TenantID, key string) (*domain.IdempotencyEntry, error) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	k := tenantID.String() + ":" + key
	if entry, ok := s.store.idempotencyKeys[k]; ok {
		return entry, nil
	}
	return nil, nil
}

// Set stores or updates an idempotency entry by tenant and key.
func (s *IdempotencyStore) Set(ctx context.Context, entry *domain.IdempotencyEntry) error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	k := entry.TenantID.String() + ":" + entry.IdempotencyKey
	s.store.idempotencyKeys[k] = entry
	return nil
}

// SetIfAbsent stores an entry only if the key is not already present.
// Returns (true, entry, nil) when inserted, or (false, existing, nil) when present.
func (s *IdempotencyStore) SetIfAbsent(ctx context.Context, entry *domain.IdempotencyEntry) (bool, *domain.IdempotencyEntry, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	k := entry.TenantID.String() + ":" + entry.IdempotencyKey
	if existing, ok := s.store.idempotencyKeys[k]; ok {
		return false, existing, nil
	}
	s.store.idempotencyKeys[k] = entry
	return true, entry, nil
}

// OutboxRepository provides non-transactional access to in-memory outbox entries.
type OutboxRepository struct {
	store *DataStore
}

// Append adds an event entry to the in-memory outbox.
func (r *OutboxRepository) Append(ctx context.Context, entry *domain.OutboxEntry) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.outboxEntries = append(r.store.outboxEntries, entry)
	return nil
}

// FetchUnpublished returns unpublished events in insertion order, up to the limit.
func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	var entries []*domain.OutboxEntry
	for _, entry := range r.store.outboxEntries {
		if entry.PublishedAt == nil {
			entries = append(entries, entry)
			if len(entries) >= limit {
				break
			}
		}
	}
	return entries, nil
}

// MarkPublished sets PublishedAt for the specified events.
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []types.EventID) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	now := time.Now()
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id.String()] = true
	}
	for _, entry := range r.store.outboxEntries {
		if idSet[entry.ID.String()] {
			entry.PublishedAt = &now
		}
	}
	return nil
}

// Verify interface implementations
var (
	_ domain.AtomicExecutor = (*DataStore)(nil)
	_ domain.Repositories   = (*DataStore)(nil)
)
