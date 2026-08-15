package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"processing/internal/decimal"
	"processing/internal/domain"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthRegisterNormalizesAndPersistsAccount(t *testing.T) {
	accounts := &fakeAccounts{}
	uow := &fakeUOW{accounts: accounts, tokens: &fakeTokens{}}
	cache := &fakeAuthCache{}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{uow}}, cache, &fakeTokenManager{})

	account, err := service.Register(context.Background(), "  User@Example.COM ", "password123", "  Test User  ", "127.0.0.1")

	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "user@example.com", account.Email)
	assert.Equal(t, "Test User", account.Name)
	assert.Equal(t, domain.RoleUser, account.Role)
	assert.True(t, uow.committed)
	require.Len(t, cache.keys, 1)
	assert.Equal(t, "auth:register:127.0.0.1", cache.keys[0])
	require.NotNil(t, accounts.created)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(accounts.created.PasswordHash), []byte("password123")))
}

func TestAuthRegisterRejectsPasswordOverBcryptLimit(t *testing.T) {
	service := newTestAuthService(&fakeTxFactory{}, &fakeAuthCache{}, &fakeTokenManager{})

	_, err := service.Register(context.Background(), "user@example.com", string(make([]byte, maxPasswordLen+1)), "Test User", "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrInvalidPassword)
}

func TestAuthLoginDoesNotMaskDatabaseFailure(t *testing.T) {
	dbErr := errors.New("database unavailable")
	readUOW := &fakeUOW{
		accounts: &fakeAccounts{getByEmailErr: dbErr},
		tokens:   &fakeTokens{},
	}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{readUOW}}, &fakeAuthCache{}, &fakeTokenManager{})

	_, err := service.Login(context.Background(), "user@example.com", "password123", "127.0.0.1")

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestAuthLoginUnknownAccountReturnsInvalidCredentials(t *testing.T) {
	readUOW := &fakeUOW{
		accounts: &fakeAccounts{getByEmailErr: domain.ErrAccountNotFound},
		tokens:   &fakeTokens{},
	}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{readUOW}}, &fakeAuthCache{}, &fakeTokenManager{})

	_, err := service.Login(context.Background(), "missing@example.com", "password123", "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestAuthLoginCommitsReadBeforePasswordCheckAndSavesSession(t *testing.T) {
	userID := uuid.New()
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	require.NoError(t, err)
	readUOW := &fakeUOW{
		accounts: &fakeAccounts{byEmail: &domain.Account{ID: userID, Email: "user@example.com", PasswordHash: string(hash), Role: domain.RoleUser}},
		tokens:   &fakeTokens{},
	}
	writeTokens := &fakeTokens{}
	writeUOW := &fakeUOW{accounts: &fakeAccounts{}, tokens: writeTokens}
	expiresAt := time.Now().Add(time.Hour)
	manager := &fakeTokenManager{generated: &domain.TokenPair{
		AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 900, JTI: "new-jti", ExpiresAt: expiresAt,
	}}
	cache := &fakeAuthCache{}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{readUOW, writeUOW}}, cache, manager)

	pair, err := service.Login(context.Background(), " USER@example.com ", "password123", "127.0.0.1")

	require.NoError(t, err)
	assert.Equal(t, "access", pair.AccessToken)
	assert.Empty(t, pair.JTI)
	assert.True(t, readUOW.committed)
	assert.True(t, writeUOW.committed)
	assert.Equal(t, "new-jti", writeTokens.savedJTI)
	assert.Equal(t, userID, writeTokens.savedUserID)
	assert.NotEqual(t, uuid.Nil, writeTokens.savedFamilyID)
	require.Len(t, cache.keys, 1)
	assert.Contains(t, cache.keys[0], "auth:login:127.0.0.1:")
}

func TestAuthLoginRollsBackWhenSessionCannotBeSaved(t *testing.T) {
	userID := uuid.New()
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	require.NoError(t, err)
	readUOW := &fakeUOW{
		accounts: &fakeAccounts{byEmail: &domain.Account{ID: userID, PasswordHash: string(hash), Role: domain.RoleUser}},
		tokens:   &fakeTokens{},
	}
	saveErr := errors.New("save failed")
	writeUOW := &fakeUOW{accounts: &fakeAccounts{}, tokens: &fakeTokens{saveErr: saveErr}}
	manager := &fakeTokenManager{generated: &domain.TokenPair{
		JTI: "new-jti", ExpiresAt: time.Now().Add(time.Hour),
	}}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{readUOW, writeUOW}}, &fakeAuthCache{}, manager)

	_, err = service.Login(context.Background(), "user@example.com", "password123", "127.0.0.1")

	assert.ErrorIs(t, err, saveErr)
	assert.False(t, writeUOW.committed)
	assert.True(t, writeUOW.rolled)
}

func TestDummyHashMatchesConfiguredBcryptCost(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyHash))
	require.NoError(t, err)
	assert.Equal(t, bcryptCost, cost)
}

func TestAuthRefreshRejectsClaimsSessionMismatch(t *testing.T) {
	userID := uuid.New()
	uow := &fakeUOW{
		accounts: &fakeAccounts{},
		tokens: &fakeTokens{session: &domain.RefreshSession{
			UserID: userID, FamilyID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour),
		}},
	}
	manager := &fakeTokenManager{claims: refreshClaims(uuid.New().String(), "old-jti")}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{uow}}, &fakeAuthCache{}, manager)

	_, err := service.Refresh(context.Background(), "refresh", "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrInvalidRefreshToken)
	assert.False(t, uow.committed)
	assert.Empty(t, uow.tokens.(*fakeTokens).revokedJTI)
}

func TestAuthRefreshRotatesSessionAtomically(t *testing.T) {
	userID := uuid.New()
	familyID := uuid.New()
	tokens := &fakeTokens{session: &domain.RefreshSession{UserID: userID, FamilyID: familyID, ExpiresAt: time.Now().Add(time.Hour)}}
	uow := &fakeUOW{
		accounts: &fakeAccounts{byID: &domain.Account{ID: userID, Role: domain.RoleUser}},
		tokens:   tokens,
	}
	manager := &fakeTokenManager{
		claims: refreshClaims(userID.String(), "old-jti"),
		generated: &domain.TokenPair{
			AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresIn: 900,
			JTI: "new-jti", ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{uow}}, &fakeAuthCache{}, manager)

	pair, err := service.Refresh(context.Background(), "refresh", "127.0.0.1")

	require.NoError(t, err)
	assert.Equal(t, "new-access", pair.AccessToken)
	assert.Equal(t, "old-jti", tokens.revokedJTI)
	assert.Equal(t, "new-jti", tokens.savedJTI)
	assert.Equal(t, familyID, tokens.savedFamilyID)
	assert.True(t, uow.committed)
}

func TestAuthRefreshReuseRevokesTokenFamily(t *testing.T) {
	userID := uuid.New()
	familyID := uuid.New()
	tokens := &fakeTokens{session: &domain.RefreshSession{
		UserID: userID, FamilyID: familyID, Revoked: true, ExpiresAt: time.Now().Add(time.Hour),
	}}
	uow := &fakeUOW{accounts: &fakeAccounts{}, tokens: tokens}
	manager := &fakeTokenManager{claims: refreshClaims(userID.String(), "old-jti")}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{uow}}, &fakeAuthCache{}, manager)

	_, err := service.Refresh(context.Background(), "refresh", "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrRefreshTokenReuse)
	assert.Equal(t, familyID, tokens.revokedFamilyID)
	assert.Equal(t, uuid.Nil, tokens.revokedAllUserID)
	assert.True(t, uow.committed)
}

func TestAuthRefreshConcurrentConsumeIsTreatedAsReuse(t *testing.T) {
	userID := uuid.New()
	familyID := uuid.New()
	tokens := &fakeTokens{
		session:   &domain.RefreshSession{UserID: userID, FamilyID: familyID, ExpiresAt: time.Now().Add(time.Hour)},
		revokeErr: domain.ErrRefreshTokenNotFound,
	}
	uow := &fakeUOW{accounts: &fakeAccounts{}, tokens: tokens}
	manager := &fakeTokenManager{claims: refreshClaims(userID.String(), "old-jti")}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{uow}}, &fakeAuthCache{}, manager)

	_, err := service.Refresh(context.Background(), "refresh", "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrRefreshTokenReuse)
	assert.Equal(t, familyID, tokens.revokedFamilyID)
	assert.Equal(t, uuid.Nil, tokens.revokedAllUserID)
	assert.True(t, uow.committed)
}

func TestAuthRefreshUsesInjectedClockForSessionExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	tokens := &fakeTokens{session: &domain.RefreshSession{UserID: userID, FamilyID: uuid.New(), ExpiresAt: now}}
	uow := &fakeUOW{accounts: &fakeAccounts{}, tokens: tokens}
	manager := &fakeTokenManager{claims: refreshClaims(userID.String(), "old-jti")}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{uow}}, &fakeAuthCache{}, manager)
	service.now = func() time.Time { return now }

	_, err := service.Refresh(context.Background(), "refresh", "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrRefreshTokenExpired)
	assert.Equal(t, "old-jti", tokens.revokedJTI)
	assert.True(t, uow.committed)
}

func TestAuthLogoutAllIsIdempotent(t *testing.T) {
	uow := &fakeUOW{accounts: &fakeAccounts{}, tokens: &fakeTokens{revokeAllErr: domain.ErrUserNotFound}}
	service := newTestAuthService(&fakeTxFactory{uows: []domain.UnitOfWork{uow}}, &fakeAuthCache{}, &fakeTokenManager{})

	err := service.LogoutAll(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.True(t, uow.committed)
}

func TestAuthRateLimitErrorKeepsDomainCause(t *testing.T) {
	service := newTestAuthService(&fakeTxFactory{}, &fakeAuthCache{err: domain.ErrRateLimited}, &fakeTokenManager{})

	_, err := service.Login(context.Background(), "user@example.com", "password123", "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrRateLimited)
}

func refreshClaims(userID, jti string) *domain.RefreshClaims {
	claims := &domain.RefreshClaims{UserID: userID}
	claims.ID = jti
	return claims
}

func newTestAuthService(tx domain.TxUOW, cache domain.Cache, manager tokenManager) *AuthService {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewAuthService(tx, cache, log, manager)
}

type fakeAuthCache struct {
	keys []string
	err  error
}

func (f *fakeAuthCache) CheckRateLimit(_ context.Context, key string) error {
	f.keys = append(f.keys, key)
	return f.err
}

func (f *fakeAuthCache) IdempotencyCheck(context.Context, string, time.Duration) error { return nil }

type fakeTokenManager struct {
	generated   *domain.TokenPair
	generateErr error
	claims      *domain.RefreshClaims
	validateErr error
}

func (f *fakeTokenManager) GenerateTokenPair(string, string) (*domain.TokenPair, error) {
	return f.generated, f.generateErr
}

func (f *fakeTokenManager) ValidateRefreshToken(string) (*domain.RefreshClaims, error) {
	return f.claims, f.validateErr
}

type fakeTxFactory struct {
	mu   sync.Mutex
	uows []domain.UnitOfWork
	err  error
}

func (f *fakeTxFactory) NewTX(context.Context) (domain.UnitOfWork, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if len(f.uows) == 0 {
		return nil, errors.New("unexpected transaction")
	}
	uow := f.uows[0]
	f.uows = f.uows[1:]
	return uow, nil
}

type fakeUOW struct {
	accounts  domain.AccountsStorage
	tokens    domain.TokenStorage
	committed bool
	rolled    bool
	commitErr error
}

func (f *fakeUOW) Accounts() domain.AccountsStorage        { return f.accounts }
func (f *fakeUOW) Transactions() domain.TransactionStorage { return fakeTransactionStorage{} }
func (f *fakeUOW) Tokens() domain.TokenStorage             { return f.tokens }
func (f *fakeUOW) Commit() error                           { f.committed = true; return f.commitErr }
func (f *fakeUOW) Rollback() error                         { f.rolled = true; return nil }

type fakeAccounts struct {
	created       *domain.Account
	createErr     error
	byID          *domain.Account
	getByIDErr    error
	byEmail       *domain.Account
	getByEmailErr error
}

func (f *fakeAccounts) Create(_ context.Context, account *domain.Account) error {
	f.created = account
	return f.createErr
}

func (f *fakeAccounts) GetById(context.Context, uuid.UUID) (*domain.Account, error) {
	return f.byID, f.getByIDErr
}

func (f *fakeAccounts) GetByEmail(context.Context, string) (*domain.Account, error) {
	return f.byEmail, f.getByEmailErr
}

func (*fakeAccounts) Sub(context.Context, uuid.UUID, decimal.Decimal) error { return nil }
func (*fakeAccounts) Add(context.Context, uuid.UUID, decimal.Decimal) error { return nil }

type fakeTokens struct {
	session          *domain.RefreshSession
	getErr           error
	savedJTI         string
	savedUserID      uuid.UUID
	savedFamilyID    uuid.UUID
	saveErr          error
	revokedJTI       string
	revokeErr        error
	revokedFamilyID  uuid.UUID
	revokeFamilyErr  error
	revokedAllUserID uuid.UUID
	revokeAllErr     error
}

func (f *fakeTokens) SaveRefreshToken(_ context.Context, jti string, userID, familyID uuid.UUID, _ time.Time) error {
	f.savedJTI = jti
	f.savedUserID = userID
	f.savedFamilyID = familyID
	return f.saveErr
}

func (f *fakeTokens) GetRefreshToken(context.Context, string) (*domain.RefreshSession, error) {
	return f.session, f.getErr
}

func (f *fakeTokens) RevokeRefreshToken(_ context.Context, jti string) error {
	f.revokedJTI = jti
	return f.revokeErr
}

func (f *fakeTokens) RevokeTokenFamily(_ context.Context, familyID uuid.UUID) error {
	f.revokedFamilyID = familyID
	return f.revokeFamilyErr
}

func (f *fakeTokens) RevokeAllUserTokens(_ context.Context, userID uuid.UUID) error {
	f.revokedAllUserID = userID
	return f.revokeAllErr
}

type fakeTransactionStorage struct{}

func (fakeTransactionStorage) Transaction(context.Context, *domain.Transaction) error { return nil }
func (fakeTransactionStorage) UpdateStatus(context.Context, *domain.Transaction, domain.TransactionStatus) error {
	return nil
}
func (fakeTransactionStorage) GetByID(context.Context, uuid.UUID) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}
func (fakeTransactionStorage) GetTransactions(context.Context, domain.TransactionFilter) ([]domain.Transaction, error) {
	return nil, nil
}
func (fakeTransactionStorage) TotalTransactions(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}
