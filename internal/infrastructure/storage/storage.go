package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"processing/internal/decimal"
	"processing/internal/domain"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type accountRepo struct {
	tx *sql.Tx
}

type txRepo struct {
	tx *sql.Tx
}

type txToken struct {
	tx *sql.Tx
}

// транзакция которую мы будем раздавать
type sqlTx struct {
	tx       *sql.Tx
	accounts *accountRepo
	token    *txToken
	txs      *txRepo
}

func (u *sqlTx) Accounts() domain.AccountsStorage        { return u.accounts }
func (u *sqlTx) Transactions() domain.TransactionStorage { return u.txs }
func (u *sqlTx) Tokens() domain.TokenStorage             { return u.token }

func (u *sqlTx) Commit() error {
	return u.tx.Commit()
}

func (u *sqlTx) Rollback() error {
	return u.tx.Rollback()
}

type uowFactory struct {
	db *sql.DB
}

func NewUoWFactory(db *sql.DB) domain.TxUOW {
	return &uowFactory{db: db}
}

// NewTX создает новую транзакцию базы данных
func (u *uowFactory) NewTX(ctx context.Context) (domain.UnitOfWork, error) {
	tx, err := u.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("tx begin: %w", err)
	}
	return &sqlTx{
		tx:       tx,
		accounts: &accountRepo{tx: tx},
		txs:      &txRepo{tx: tx},
		token:    &txToken{tx: tx},
	}, nil
}

// Create - создаёт аккаунт и возвращает ID
func (s *accountRepo) Create(ctx context.Context, ac *domain.Account) error {
	query := `INSERT INTO accounts(id, name, email, balance, password_hash, role) VALUES($1, $2, $3, $4, $5, $6)`

	if _, err := s.tx.ExecContext(ctx, query, ac.ID, ac.Name, ac.Email, ac.Balance, ac.PasswordHash, ac.Role); err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == "23505" {
			return domain.ErrAccountAlreadyExist
		}
		return fmt.Errorf("создание аккакунта: %w", err)
	}
	return nil
}

// GetById - возвращает аккаунт по id
func (s *accountRepo) GetById(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	ac := &domain.Account{}
	query := `SELECT id, name, email, balance, password_hash, role FROM accounts WHERE id = $1`
	err := s.tx.QueryRowContext(ctx, query, id).Scan(&ac.ID, &ac.Name, &ac.Email, &ac.Balance, &ac.PasswordHash, &ac.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("получение данных аккаунта по id: %w", err)
	}
	return ac, nil
}

// GetByEmail - возвращает аккаунт по email
func (s *accountRepo) GetByEmail(ctx context.Context, email string) (*domain.Account, error) {
	ac := &domain.Account{}
	query := `SELECT id, name, email, balance, password_hash, role FROM accounts WHERE LOWER(email) = LOWER($1)`
	err := s.tx.QueryRowContext(ctx, query, email).Scan(&ac.ID, &ac.Name, &ac.Email, &ac.Balance, &ac.PasswordHash, &ac.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("получение данных аккаунта по email: %w", err)
	}
	return ac, nil
}

// Sub - вычетает сумму с баланса аккаунта
func (s *accountRepo) Sub(ctx context.Context, sender_id uuid.UUID, amount decimal.Decimal) error {
	query := `
	UPDATE accounts
	SET balance = balance - $1
	WHERE id = $2 AND balance >= $1
	`
	res, err := s.tx.ExecContext(ctx, query, amount, sender_id)
	if err != nil {
		return fmt.Errorf("вычет суммы с баланса: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrInsufficientFunds
	}

	return nil
}

// Add - добавляет сумму на баланс аккаунта
func (s *accountRepo) Add(ctx context.Context, receiver_id uuid.UUID, amount decimal.Decimal) error {
	query := `UPDATE accounts SET balance = balance + $1 WHERE id = $2`
	res, err := s.tx.ExecContext(ctx, query, amount, receiver_id)
	if err != nil {
		return fmt.Errorf("добавление суммы на баланс: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return domain.ErrReceiverAccountNotFound
	}

	return nil
}

// Transaction создает транзакцию в бд
func (s *txRepo) Transaction(ctx context.Context, tx *domain.Transaction) error {
	query := `
	INSERT INTO transactions(id, amount, sender_id, receiver_id) VALUES($1, $2, $3, $4)
	RETURNING status, created_at
	`
	if err := s.tx.QueryRowContext(ctx, query, tx.ID, tx.Amount, tx.Sender_id, tx.Receiver_id).
		Scan(&tx.Status, &tx.Created_at); err != nil {
		return fmt.Errorf("создание транзакции: %w", err)
	}
	return nil
}

// UpdateStatus обновляет статус транзакции в бд
func (s *txRepo) UpdateStatus(ctx context.Context, tx *domain.Transaction, status domain.TransactionStatus) error {
	query := `
	UPDATE transactions SET status = $1 WHERE id = $2
	RETURNING status, created_at
	`
	if err := s.tx.QueryRowContext(ctx, query, status, tx.ID).Scan(&tx.Status, &tx.Created_at); err != nil {
		return fmt.Errorf("обновление статуса транзакции: %w", err)
	}
	return nil
}

func (s *accountRepo) LockForTransfer(ctx context.Context, firstID, secondID uuid.UUID) error {
	rows, err := s.tx.QueryContext(ctx, `
		SELECT id
		FROM accounts
		WHERE id IN ($1, $2)
		ORDER BY id
		FOR UPDATE
	`, firstID, secondID)
	if err != nil {
		return fmt.Errorf("lock transfer accounts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan locked account: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate locked accounts: %w", err)
	}
	return nil
}

func (s *txRepo) TryCreateIdempotency(ctx context.Context, record *domain.TransferIdempotency) (bool, error) {
	query := `
	INSERT INTO transfer_idempotency(sender_id, idempotency_key, request_fingerprint, status)
	VALUES($1, $2, $3, 'processing')
	ON CONFLICT (sender_id, idempotency_key) DO NOTHING
	RETURNING status, created_at, updated_at
	`

	err := s.tx.QueryRowContext(
		ctx,
		query,
		record.SenderID,
		record.Key,
		record.RequestFingerprint,
	).Scan(&record.Status, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("создание записи идемпотентности: %w", err)
	}

	return true, nil
}

func (s *txRepo) GetIdempotency(ctx context.Context, senderID uuid.UUID, key string) (domain.TransferIdempotency, error) {
	record := domain.TransferIdempotency{}
	transactionID := uuid.NullUUID{}
	query := `
	SELECT sender_id, idempotency_key, request_fingerprint, status, transaction_id, created_at, updated_at
	FROM transfer_idempotency
	WHERE sender_id = $1 AND idempotency_key = $2
	`

	err := s.tx.QueryRowContext(ctx, query, senderID, key).Scan(
		&record.SenderID,
		&record.Key,
		&record.RequestFingerprint,
		&record.Status,
		&transactionID,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return domain.TransferIdempotency{}, fmt.Errorf("получение записи идемпотентности: %w", err)
	}

	if transactionID.Valid {
		id := transactionID.UUID
		record.TransactionID = &id
	}

	return record, nil
}

func (s *txRepo) CompleteIdempotency(ctx context.Context, senderID uuid.UUID, key string, transactionID uuid.UUID) error {
	query := `
	UPDATE transfer_idempotency
	SET status = 'completed', transaction_id = $3, updated_at = now()
	WHERE sender_id = $1 AND idempotency_key = $2 AND status = 'processing'
	`

	result, err := s.tx.ExecContext(ctx, query, senderID, key, transactionID)
	if err != nil {
		return fmt.Errorf("завершение записи идемпотентности: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("проверка завершения записи идемпотентности: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("запись идемпотентности не найдена или уже завершена")
	}

	return nil
}

func (s *txRepo) GetByID(ctx context.Context, transactionID uuid.UUID) (domain.Transaction, error) {
	transaction := domain.Transaction{}
	query := `SELECT id, amount, sender_id, receiver_id, status, created_at FROM transactions WHERE id = $1`
	err := s.tx.
		QueryRowContext(ctx, query, transactionID).
		Scan(
			&transaction.ID,
			&transaction.Amount,
			&transaction.Sender_id,
			&transaction.Receiver_id,
			&transaction.Status,
			&transaction.Created_at,
		)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("получение транзакции по айди: %w", err)
	}
	return transaction, nil
}

// GetTransactions получает транзакции по фильтрам
func (s *txRepo) GetTransactions(ctx context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	query, args := sqlrequest(ctx, filter)
	if query == "" {
		return nil, errors.New("не получилось построить запрос")
	}

	rows, err := s.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("получение транзакций по фильтрам: %w", err)
	}
	defer rows.Close()

	transactions := []domain.Transaction{}
	for rows.Next() {
		var t domain.Transaction
		err := rows.Scan(
			&t.ID,
			&t.Amount,
			&t.Sender_id,
			&t.Receiver_id,
			&t.Status,
			&t.Created_at,
		)
		if err != nil {
			return nil, fmt.Errorf("сканирование транзакции: %w", err)
		}
		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("обработка строк результата: %w", err)
	}

	return transactions, nil
}

func (s *txRepo) TotalTransactions(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM transactions WHERE receiver_id=$1 OR sender_id=$1`
	var count int

	if err := s.tx.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (s *txToken) SaveRefreshToken(ctx context.Context, jti string, userID, familyID uuid.UUID, expiresAt time.Time) error {
	query := `INSERT INTO refresh_token(jti, user_id, family_id, expires_at, revoked) VALUES($1, $2, $3, $4, false)`

	res, err := s.tx.ExecContext(ctx, query, jti, userID, familyID, expiresAt)
	if err != nil {
		return domain.ErrSaveRefreshToken
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return domain.ErrSaveRefreshToken
	}

	return nil
}

func (s *txToken) GetRefreshToken(ctx context.Context, jti string) (*domain.RefreshSession, error) {
	query := `SELECT user_id, family_id, revoked, expires_at FROM refresh_token WHERE jti = $1`

	session := &domain.RefreshSession{}
	err := s.tx.QueryRowContext(ctx, query, jti).Scan(&session.UserID, &session.FamilyID, &session.Revoked, &session.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("получение refresh токена: %w", err)
	}

	return session, nil
}

func (s *txToken) RevokeRefreshToken(ctx context.Context, jti string) error {
	query := `UPDATE refresh_token SET revoked = true WHERE jti = $1 AND revoked = false`

	res, err := s.tx.ExecContext(ctx, query, jti)
	if err != nil {
		return fmt.Errorf("отзыв refresh токена: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return domain.ErrRefreshTokenNotFound
	}

	return nil
}

func (s *txToken) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE refresh_token SET revoked = true WHERE user_id = $1 AND revoked = false`

	res, err := s.tx.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("отзыв всех токенов пользователя: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

func (s *txToken) RevokeTokenFamily(ctx context.Context, familyID uuid.UUID) error {
	query := `UPDATE refresh_token SET revoked = true WHERE family_id = $1 AND revoked = false`

	_, err := s.tx.ExecContext(ctx, query, familyID)
	if err != nil {
		return fmt.Errorf("отзыв семейства refresh токенов: %w", err)
	}

	return nil
}
