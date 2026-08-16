package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"processing/internal/decimal"
	"processing/internal/domain"
	"processing/internal/infrastructure/logger"

	"github.com/google/uuid"
)

const transferOperationVersion = "transfer:v1"

type transferCommand struct {
	SenderID           uuid.UUID
	ReceiverID         uuid.UUID
	Key                string
	Amount             decimal.Decimal
	RequestFingerprint string
}

type TransactionsService struct {
	tx    domain.TxUOW
	cache domain.Cache
	log   *slog.Logger
}

func NewTransactionsService(txUOW domain.TxUOW, cache domain.Cache, log *slog.Logger) *TransactionsService {
	log = logger.WithService(log, "Transactions")
	return &TransactionsService{
		tx:    txUOW,
		cache: cache,
		log:   log,
	}
}

// Transfer атомарно меняет балансы, создаёт транзакцию и сохраняет
// результат по Idempotency-Key в одной SQL-транзакции.
func (ts *TransactionsService) Transfer(
	ctx context.Context,
	sender_id, receiver_id uuid.UUID,
	key string,
	amount decimal.Decimal,
) (string, error) {
	if err := domain.ValidateIdempotencyKey(key); err != nil {
		return "", err
	}

	command := transferCommand{
		SenderID:   sender_id,
		ReceiverID: receiver_id,
		Key:        key,
		Amount:     amount,
	}
	command.RequestFingerprint = transferFingerprint(command)

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка создания транзакции БД", "err", err)
		return "", err
	}

	defer uow.Rollback()

	replayID, err := ts.resolveIdempotency(ctx, uow.Transactions(), command)
	if err != nil {
		return "", err
	}
	if replayID != nil {
		return replayID.String(), nil
	}

	if err := ts.cache.CheckRateLimit(ctx, command.SenderID.String()); err != nil {
		ts.log.WarnContext(ctx, "превышен лимит запросов при переводе", "sender_id", command.SenderID)
		return "", err
	}

	tx, err := ts.executeTransfer(ctx, uow, command)
	if err != nil {
		return "", err
	}

	if err := uow.Transactions().CompleteIdempotency(ctx, command.SenderID, command.Key, tx.ID); err != nil {
		ts.log.ErrorContext(ctx, "ошибка сохранения результа идемпотентности", "err", err, "transaction_id", tx.ID)
		return "", err
	}

	if err := uow.Commit(); err != nil {
		ts.log.ErrorContext(ctx, "ошибка коммита транзакции БД", "err", err, "transaction_id", tx.ID)
		return "", err
	}

	ts.log.InfoContext(ctx, "транзакция успешно завершена", "transaction_id", tx.ID, "sender_id", command.SenderID, "receiver_id", command.ReceiverID, "amount", command.Amount)
	return tx.ID.String(), nil
}

func (ts *TransactionsService) resolveIdempotency(
	ctx context.Context,
	repo domain.TransactionStorage,
	command transferCommand,
) (*uuid.UUID, error) {
	record := &domain.TransferIdempotency{
		SenderID:           command.SenderID,
		Key:                command.Key,
		RequestFingerprint: command.RequestFingerprint,
	}
	created, err := repo.TryCreateIdempotency(ctx, record)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка резервирования Idempotency-Key", "err", err, "sender_id", command.SenderID)
		return nil, err
	}
	if created {
		return nil, nil
	}

	existing, err := repo.GetIdempotency(ctx, command.SenderID, command.Key)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения результата идемпотентности", "err", err, "sender_id", command.SenderID)
		return nil, err
	}
	if existing.RequestFingerprint != command.RequestFingerprint {
		ts.log.WarnContext(ctx, "Idempotency-Key повторно использован с другим payload", "sender_id", command.SenderID)
		return nil, domain.ErrIdempotencyConflict
	}
	if existing.Status != domain.IdempotencyStatusCompleted || existing.TransactionID == nil {
		return nil, domain.ErrIdempotencyInProgress
	}

	ts.log.InfoContext(ctx, "возвращён результат повторного перевода", "transaction_id", *existing.TransactionID, "sender_id", command.SenderID)
	return existing.TransactionID, nil
}

func (ts *TransactionsService) executeTransfer(
	ctx context.Context,
	uow domain.UnitOfWork,
	command transferCommand,
) (*domain.Transaction, error) {
	sender, err := uow.Accounts().GetById(ctx, command.SenderID)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения аккаунта отправителя", "err", err, "sender_id", command.SenderID)
		return nil, err
	}
	receiver, err := uow.Accounts().GetById(ctx, command.ReceiverID)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения аккаунта получателя", "err", err, "receiver_id", command.ReceiverID)
		return nil, err
	}

	if sender.ID == receiver.ID {
		ts.log.WarnContext(ctx, "попытка перевода на собственный счет", "sender_id", command.SenderID)
		return nil, domain.ErrSameAccount
	}
	if !command.Amount.IsPositive() {
		ts.log.WarnContext(ctx, "попытка перевода отрицательной суммы", "sender_id", command.SenderID, "amount", command.Amount)
		return nil, domain.ErrInvalidAmount
	}

	if err := uow.Accounts().Sub(ctx, command.SenderID, command.Amount); err != nil {
		ts.log.ErrorContext(ctx, "ошибка вычисления суммы со счета отправителя", "err", err, "sender_id", command.SenderID, "amount", command.Amount)
		return nil, err
	}
	if err := uow.Accounts().Add(ctx, command.ReceiverID, command.Amount); err != nil {
		ts.log.ErrorContext(ctx, "ошибка добавления суммы на счет получателя", "err", err, "receiver_id", command.ReceiverID, "amount", command.Amount)
		return nil, err
	}

	tx, err := domain.NewTransaction(command.Amount, command.SenderID, command.ReceiverID)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка создания объекта транзакции", "err", err, "sender_id", command.SenderID, "receiver_id", command.ReceiverID)
		return nil, err
	}
	if err := uow.Transactions().Transaction(ctx, tx); err != nil {
		ts.log.ErrorContext(ctx, "ошибка сохранения транзакции в БД", "err", err, "transaction_id", tx.ID)
		return nil, err
	}
	if err := uow.Transactions().UpdateStatus(ctx, tx, domain.StatusCompleted); err != nil {
		ts.log.ErrorContext(ctx, "ошибка обновления статуса транзакции", "err", err, "transaction_id", tx.ID)
		return nil, err
	}

	return tx, nil
}

func transferFingerprint(command transferCommand) string {
	payload := transferOperationVersion + "\x00" + command.SenderID.String() + "\x00" + command.ReceiverID.String() + "\x00" + command.Amount.String()
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (ts *TransactionsService) GetTransaction(
	ctx context.Context,
	transactionID,
	userID uuid.UUID,
	key string,
) (domain.Transaction, error) {
	if err := ts.cache.CheckRateLimit(ctx, userID.String()); err != nil {
		ts.log.WarnContext(ctx, "превышен лимит запросов при получении транзакции", "user_id", userID)
		return domain.Transaction{}, err
	}

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка создания транзакции БД", "err", err)
		return domain.Transaction{}, err
	}
	defer uow.Rollback()

	transaction, err := uow.Transactions().GetByID(ctx, transactionID)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения транзакции из БД", "err", err, "transaction_id", transactionID)
		return domain.Transaction{}, err
	}

	if transaction.Sender_id != userID && transaction.Receiver_id != userID {
		ts.log.WarnContext(ctx, "попытка доступа к чужой транзакции", "user_id", userID, "transaction_id", transactionID)
		return domain.Transaction{}, domain.ErrAccessDenied
	}

	if err := uow.Commit(); err != nil {
		ts.log.ErrorContext(ctx, "ошибка коммита транзакции БД", "err", err)
		return domain.Transaction{}, err
	}

	return transaction, nil
}

func (ts *TransactionsService) GetTransactionFilter(
	ctx context.Context,
	t *domain.TransactionFilter,
	userID uuid.UUID,
	key string,
) ([]domain.Transaction, error) {
	if err := ts.cache.CheckRateLimit(ctx, userID.String()); err != nil {
		ts.log.WarnContext(ctx, "превышен лимит запросов при фильтрации транзакций", "user_id", userID)
		return nil, err
	}

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка создания транзакции БД", "err", err)
		return nil, err
	}
	defer uow.Rollback()

	transactions, err := uow.Transactions().GetTransactions(ctx, *t)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения транзакций из БД", "err", err, "account_id", t.AccountID)
		return nil, err
	}

	if err := uow.Commit(); err != nil {
		ts.log.ErrorContext(ctx, "ошибка коммита транзакции БД", "err", err)
		return nil, err
	}

	ts.log.InfoContext(ctx, "транзакции по фильтру получены", "user_id", userID, "count", len(transactions))
	return transactions, nil
}
