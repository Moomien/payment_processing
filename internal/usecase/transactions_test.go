package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"processing/internal/decimal"
	"processing/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransferPersistsIdempotencyWithMoneyMovement(t *testing.T) {
	senderID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	receiverID := uuid.MustParse("10000000-0000-0000-0000-000000000002")
	amount, err := decimal.NewFromString("125.50")
	require.NoError(t, err)

	repo := &transferUnitTxRepo{createIdempotency: true}
	accounts := transferUnitAccounts(senderID, receiverID)
	uow := &transferUnitUOW{accounts: accounts, transactions: repo}
	cache := &transferUnitCache{}
	service := newTransferUnitService(&transferUnitFactory{uows: []*transferUnitUOW{uow}}, cache)

	transactionID, err := service.Transfer(context.Background(), senderID, receiverID, "transfer-key-1", amount)

	require.NoError(t, err)
	require.NotEmpty(t, transactionID)
	require.NotNil(t, repo.savedTransaction)
	assert.Equal(t, repo.savedTransaction.ID.String(), transactionID)
	assert.Equal(t, repo.savedTransaction.ID, repo.completedTransactionID)
	assert.Equal(t, senderID, repo.reserved.SenderID)
	assert.Equal(t, "transfer-key-1", repo.reserved.Key)
	assert.Len(t, repo.reserved.RequestFingerprint, 64)
	assert.Equal(t, 1, accounts.subCalls)
	assert.Equal(t, 1, accounts.addCalls)
	assert.Equal(t, 1, cache.rateLimitCalls)
	assert.True(t, uow.committed)
}

func TestTransferReplayReturnsStoredResultBeforeRedis(t *testing.T) {
	senderID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	receiverID := uuid.MustParse("20000000-0000-0000-0000-000000000002")
	transactionID := uuid.MustParse("20000000-0000-0000-0000-000000000003")
	amount, err := decimal.NewFromString("10.00")
	require.NoError(t, err)

	repo := &transferUnitTxRepo{
		createIdempotency: false,
		existing: domain.TransferIdempotency{
			SenderID:           senderID,
			Key:                "replay-key",
			RequestFingerprint: transferFingerprint(transferCommand{
				SenderID:   senderID,
				ReceiverID: receiverID,
				Amount:     amount,
			}),
			Status:             domain.IdempotencyStatusCompleted,
			TransactionID:      &transactionID,
		},
	}
	cache := &transferUnitCache{rateLimitErr: errors.New("redis unavailable")}
	uow := &transferUnitUOW{transactions: repo}
	service := newTransferUnitService(&transferUnitFactory{uows: []*transferUnitUOW{uow}}, cache)

	result, err := service.Transfer(context.Background(), senderID, receiverID, "replay-key", amount)

	require.NoError(t, err)
	assert.Equal(t, transactionID.String(), result)
	assert.Equal(t, 0, cache.rateLimitCalls)
	assert.Nil(t, repo.savedTransaction)
	assert.False(t, uow.committed)
}

func TestTransferRejectsSameKeyWithDifferentPayload(t *testing.T) {
	senderID := uuid.MustParse("30000000-0000-0000-0000-000000000001")
	receiverID := uuid.MustParse("30000000-0000-0000-0000-000000000002")
	transactionID := uuid.MustParse("30000000-0000-0000-0000-000000000003")
	amount, err := decimal.NewFromString("20")
	require.NoError(t, err)

	repo := &transferUnitTxRepo{
		existing: domain.TransferIdempotency{
			SenderID:           senderID,
			Key:                "conflict-key",
			RequestFingerprint: "different-fingerprint",
			Status:             domain.IdempotencyStatusCompleted,
			TransactionID:      &transactionID,
		},
	}
	cache := &transferUnitCache{}
	uow := &transferUnitUOW{transactions: repo}
	service := newTransferUnitService(&transferUnitFactory{uows: []*transferUnitUOW{uow}}, cache)

	_, err = service.Transfer(context.Background(), senderID, receiverID, "conflict-key", amount)

	require.ErrorIs(t, err, domain.ErrIdempotencyConflict)
	assert.Equal(t, 0, cache.rateLimitCalls)
	assert.Nil(t, repo.savedTransaction)
	assert.False(t, uow.committed)
}

func TestTransferRollbackAllowsRetry(t *testing.T) {
	senderID := uuid.MustParse("40000000-0000-0000-0000-000000000001")
	receiverID := uuid.MustParse("40000000-0000-0000-0000-000000000002")
	amount, err := decimal.NewFromString("50")
	require.NoError(t, err)

	failedRepo := &transferUnitTxRepo{createIdempotency: true}
	failedAccounts := transferUnitAccounts(senderID, receiverID)
	failedAccounts.subErr = domain.ErrInsufficientFunds
	failedUOW := &transferUnitUOW{accounts: failedAccounts, transactions: failedRepo}

	successRepo := &transferUnitTxRepo{createIdempotency: true}
	successUOW := &transferUnitUOW{
		accounts:     transferUnitAccounts(senderID, receiverID),
		transactions: successRepo,
	}

	service := newTransferUnitService(
		&transferUnitFactory{uows: []*transferUnitUOW{failedUOW, successUOW}},
		&transferUnitCache{},
	)

	_, err = service.Transfer(context.Background(), senderID, receiverID, "retry-key", amount)
	require.ErrorIs(t, err, domain.ErrInsufficientFunds)
	assert.True(t, failedUOW.rolledBack)
	assert.False(t, failedUOW.committed)

	transactionID, err := service.Transfer(context.Background(), senderID, receiverID, "retry-key", amount)
	require.NoError(t, err)
	assert.NotEmpty(t, transactionID)
	assert.True(t, successUOW.committed)
}

func TestTransferValidatesKeyBeforeOpeningTransaction(t *testing.T) {
	service := newTransferUnitService(&transferUnitFactory{}, &transferUnitCache{})
	amount, err := decimal.NewFromString("1")
	require.NoError(t, err)

	_, err = service.Transfer(context.Background(), uuid.New(), uuid.New(), "", amount)

	require.ErrorIs(t, err, domain.ErrIdempotencyKeyRequired)
}

func TestTransferFingerprintCanonicalizesAmount(t *testing.T) {
	senderID := uuid.MustParse("50000000-0000-0000-0000-000000000001")
	receiverID := uuid.MustParse("50000000-0000-0000-0000-000000000002")
	plain, err := decimal.NewFromString("100.00")
	require.NoError(t, err)
	scientific, err := decimal.NewFromString("1e2")
	require.NoError(t, err)

	assert.Equal(
		t,
		transferFingerprint(transferCommand{SenderID: senderID, ReceiverID: receiverID, Amount: plain}),
		transferFingerprint(transferCommand{SenderID: senderID, ReceiverID: receiverID, Amount: scientific}),
	)
}

func newTransferUnitService(factory domain.TxUOW, cache domain.Cache) *TransactionsService {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewTransactionsService(factory, cache, log)
}

func transferUnitAccounts(senderID, receiverID uuid.UUID) *transferUnitAccountRepo {
	return &transferUnitAccountRepo{accounts: map[uuid.UUID]*domain.Account{
		senderID:   {ID: senderID},
		receiverID: {ID: receiverID},
	}}
}

type transferUnitCache struct {
	rateLimitCalls int
	rateLimitErr   error
}

func (f *transferUnitCache) CheckRateLimit(context.Context, string) error {
	f.rateLimitCalls++
	return f.rateLimitErr
}

func (*transferUnitCache) IdempotencyCheck(context.Context, string, time.Duration) error {
	return errors.New("transfer must not use Redis idempotency")
}

type transferUnitFactory struct {
	uows []*transferUnitUOW
}

func (f *transferUnitFactory) NewTX(context.Context) (domain.UnitOfWork, error) {
	if len(f.uows) == 0 {
		return nil, errors.New("unexpected transaction")
	}
	uow := f.uows[0]
	f.uows = f.uows[1:]
	return uow, nil
}

type transferUnitUOW struct {
	accounts     domain.AccountsStorage
	transactions domain.TransactionStorage
	committed    bool
	rolledBack   bool
}

func (f *transferUnitUOW) Accounts() domain.AccountsStorage { return f.accounts }
func (f *transferUnitUOW) Transactions() domain.TransactionStorage {
	return f.transactions
}
func (*transferUnitUOW) Tokens() domain.TokenStorage { return nil }
func (f *transferUnitUOW) Commit() error {
	f.committed = true
	return nil
}
func (f *transferUnitUOW) Rollback() error {
	f.rolledBack = true
	return nil
}

type transferUnitAccountRepo struct {
	accounts map[uuid.UUID]*domain.Account
	subCalls int
	addCalls int
	subErr   error
	addErr   error
}

func (*transferUnitAccountRepo) Create(context.Context, *domain.Account) error { return nil }
func (f *transferUnitAccountRepo) GetById(_ context.Context, id uuid.UUID) (*domain.Account, error) {
	account, ok := f.accounts[id]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return account, nil
}
func (*transferUnitAccountRepo) GetByEmail(context.Context, string) (*domain.Account, error) {
	return nil, domain.ErrAccountNotFound
}
func (f *transferUnitAccountRepo) Sub(context.Context, uuid.UUID, decimal.Decimal) error {
	f.subCalls++
	return f.subErr
}
func (f *transferUnitAccountRepo) Add(context.Context, uuid.UUID, decimal.Decimal) error {
	f.addCalls++
	return f.addErr
}

type transferUnitTxRepo struct {
	createIdempotency    bool
	createIdempotencyErr error
	reserved             domain.TransferIdempotency
	existing             domain.TransferIdempotency
	getIdempotencyErr    error
	savedTransaction     *domain.Transaction
	transactionErr       error
	updateStatusErr      error
	completeErr          error
	completedTransactionID uuid.UUID
}

func (f *transferUnitTxRepo) Transaction(_ context.Context, transaction *domain.Transaction) error {
	f.savedTransaction = transaction
	return f.transactionErr
}
func (f *transferUnitTxRepo) UpdateStatus(_ context.Context, transaction *domain.Transaction, status domain.TransactionStatus) error {
	transaction.Status = status
	return f.updateStatusErr
}
func (f *transferUnitTxRepo) TryCreateIdempotency(_ context.Context, record *domain.TransferIdempotency) (bool, error) {
	f.reserved = *record
	return f.createIdempotency, f.createIdempotencyErr
}
func (f *transferUnitTxRepo) GetIdempotency(context.Context, uuid.UUID, string) (domain.TransferIdempotency, error) {
	return f.existing, f.getIdempotencyErr
}
func (f *transferUnitTxRepo) CompleteIdempotency(_ context.Context, _ uuid.UUID, _ string, transactionID uuid.UUID) error {
	f.completedTransactionID = transactionID
	return f.completeErr
}
func (*transferUnitTxRepo) GetByID(context.Context, uuid.UUID) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}
func (*transferUnitTxRepo) GetTransactions(context.Context, domain.TransactionFilter) ([]domain.Transaction, error) {
	return nil, nil
}
func (*transferUnitTxRepo) TotalTransactions(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}
