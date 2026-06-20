package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"processing/internal/decimal"
	"processing/internal/domain"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrAccountAlreadyExist = errors.New("Account already exists")
)

type accountRepo struct {
	tx  *sql.Tx
	log *slog.Logger
}

type txRepo struct {
	tx  *sql.Tx
	log *slog.Logger
}

type txToken struct {
	tx  *sql.Tx
	log *slog.Logger
}

// транзакция которую мы будем раздавать
type sqlTx struct {
	tx       *sql.Tx
	accounts *accountRepo
	token    *txToken
	txs      *txRepo
	log      *slog.Logger
}

func (u *sqlTx) Accounts() domain.AccountsStorage        { return u.accounts }
func (u *sqlTx) Transactions() domain.TransactionStorage { return u.txs }
func (u *sqlTx) Tokens() domain.TokenStorage             { return u.token }

func (u *sqlTx) Commit() error {
	err := u.tx.Commit()
	if err != nil {
		u.log.Error("ошибка при коммите транзакции", "error", err)
		return err
	}
	u.log.Debug("транзакция успешно закоммичена")
	return nil
}

func (u *sqlTx) Rollback() error {
	err := u.tx.Rollback()
	if err != nil {
		u.log.Error("ошибка при откате транзакции", "error", err)
		return err
	}
	u.log.Debug("транзакция успешно откачена")
	return nil
}

type uowFactory struct {
	db  *sql.DB
	log *slog.Logger
}

func NewUoWFactory(db *sql.DB, log *slog.Logger) domain.TxUOW {
	return &uowFactory{db: db, log: log}
}

// NewTX создает новую транзакцию базы данных
func (u *uowFactory) NewTX(ctx context.Context) (domain.UnitOfWork, error) {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		u.log.ErrorContext(ctx, "не удалось начать транзакцию", "error", err)
		return nil, fmt.Errorf("tx begin: %w", err)
	}
	u.log.DebugContext(ctx, "транзакция успешно создана")
	return &sqlTx{
		tx:       tx,
		accounts: &accountRepo{tx: tx, log: u.log},
		txs:      &txRepo{tx: tx, log: u.log},
		token:    &txToken{tx: tx, log: u.log},
		log:      u.log,
	}, nil
}

// Create - создаёт аккаунт и возвращает ID
func (s *accountRepo) Create(ctx context.Context, ac *domain.Account) error {
	query := `INSERT INTO accounts(id, name, email, balance, password_hash, role) VALUES($1, $2, $3, $4, $5, $6)`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)
	s.log.DebugContext(ctx, "создание аккаунта", "account_id", ac.ID, "name", ac.Name, "email", ac.Email, "balance", ac.Balance, "role", ac.Role)

	if _, err := s.tx.ExecContext(ctx, query, ac.ID, ac.Name, ac.Email, ac.Balance, ac.PasswordHash, ac.Role); err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == "23505" {
			return ErrAccountAlreadyExist
		}
		s.log.ErrorContext(ctx, "ошибка создания аккаунта", "error", err, "account_id", ac.ID, "email", ac.Email)
		return fmt.Errorf("создание аккакунта: %w", err)
	}
	s.log.InfoContext(ctx, "аккаунт успешно создан", "account_id", ac.ID)
	return nil
}

// GetById - возвращает аккаунт по id
func (s *accountRepo) GetById(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	s.log.DebugContext(ctx, "получение аккаунта по id", "account_id", id)
	ac := &domain.Account{}
	query := `SELECT id, name, email, balance, password_hash, role FROM accounts WHERE id = $1`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)
	err := s.tx.QueryRowContext(ctx, query, id).Scan(&ac.ID, &ac.Name, &ac.Email, &ac.Balance, &ac.PasswordHash, &ac.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.log.WarnContext(ctx, "аккаунт не найден", "account_id", id)
			return nil, fmt.Errorf("аккаунт не найден или не создан: %w", err)
		}
		s.log.ErrorContext(ctx, "ошибка получения аккаунта", "error", err, "account_id", id)
		return nil, fmt.Errorf("получение данных аккаунта по id: %w", err)
	}
	s.log.DebugContext(ctx, "аккаунт успешно получен", "account_id", id, "balance", ac.Balance)
	return ac, nil
}

// GetByEmail - возвращает аккаунт по email
func (s *accountRepo) GetByEmail(ctx context.Context, email string) (*domain.Account, error) {
	s.log.DebugContext(ctx, "получение аккаунта по email", "email", email)
	ac := &domain.Account{}
	query := `SELECT id, name, email, balance, password_hash, role FROM accounts WHERE email = $1`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)
	err := s.tx.QueryRowContext(ctx, query, email).Scan(&ac.ID, &ac.Name, &ac.Email, &ac.Balance, &ac.PasswordHash, &ac.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.log.WarnContext(ctx, "аккаунт не найден", "email", email)
			return nil, fmt.Errorf("аккаунт не найден: %w", err)
		}
		s.log.ErrorContext(ctx, "ошибка получения аккаунта по email", "error", err, "email", email)
		return nil, fmt.Errorf("получение данных аккаунта по email: %w", err)
	}
	s.log.DebugContext(ctx, "аккаунт успешно получен по email", "email", email, "user_id", ac.ID)
	return ac, nil
}

// Sub - вычетает сумму с баланса аккаунта
func (s *accountRepo) Sub(ctx context.Context, sender_id uuid.UUID, amount decimal.Decimal) error {
	s.log.DebugContext(ctx, "вычет суммы с баланса", "account_id", sender_id, "amount", amount)
	query := `
	UPDATE accounts 
	SET balance = balance - $1 
	WHERE id = $2 AND balance >= $1
	`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)
	res, err := s.tx.ExecContext(ctx, query, amount, sender_id)
	if err != nil {
		s.log.ErrorContext(ctx, "ошибка вычета суммы с баланса", "error", err, "account_id", sender_id, "amount", amount)
		return fmt.Errorf("вычет суммы с баланса: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.log.ErrorContext(ctx, "вычет суммы с баланса", "err", err)
		return err
	}
	if rows == 0 {
		return domain.ErrInsufficientFunds
	}

	s.log.InfoContext(ctx, "сумма успешно вычтена с баланса", "account_id", sender_id, "amount", amount)
	return nil
}

// Add - добавляет сумму на баланс аккаунта
func (s *accountRepo) Add(ctx context.Context, receiver_id uuid.UUID, amount decimal.Decimal) error {
	s.log.DebugContext(ctx, "добавление суммы на баланс", "account_id", receiver_id, "amount", amount)
	query := `UPDATE accounts SET balance = balance + $1 WHERE id = $2`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)
	res, err := s.tx.ExecContext(ctx, query, amount, receiver_id)
	if err != nil {
		s.log.ErrorContext(ctx, "ошибка добавления суммы на баланс", "error", err, "account_id", receiver_id, "amount", amount)
		return fmt.Errorf("добавление суммы на баланс: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.log.ErrorContext(ctx, "добавление суммы на баланс", "err", err)
		return err
	}

	if rows == 0 {
		return domain.ErrReceiverAccountNotFound
	}

	s.log.InfoContext(ctx, "сумма успешно добавлена на баланс", "account_id", receiver_id, "amount", amount)
	return nil
}

// Transaction создает транзакцию в бд
func (s *txRepo) Transaction(ctx context.Context, tx *domain.Transaction) error {
	s.log.DebugContext(ctx, "создание транзакции", "transaction_id", tx.ID, "amount", tx.Amount, "sender_id", tx.Sender_id, "receiver_id", tx.Receiver_id)
	query := `
	INSERT INTO transactions(id, amount, sender_id, receiver_id) VALUES($1, $2, $3, $4)
	RETURNING status, created_at
	`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)
	if err := s.tx.QueryRowContext(ctx, query, tx.ID, tx.Amount, tx.Sender_id, tx.Receiver_id).
		Scan(&tx.Status, &tx.Created_at); err != nil {
		s.log.ErrorContext(ctx, "ошибка создания транзакции", "error", err, "transaction_id", tx.ID)
		return fmt.Errorf("создание транзакции: %w", err)
	}
	s.log.InfoContext(ctx, "транзакция успешно создана", "transaction_id", tx.ID, "status", tx.Status)
	return nil
}

// UpdateStatus обновляет статус транзакции в бд
func (s *txRepo) UpdateStatus(ctx context.Context, tx *domain.Transaction, status domain.TransactionStatus) error {
	s.log.DebugContext(ctx, "обновление статуса транзакции", "transaction_id", tx.ID, "new_status", status)
	query := `
	UPDATE transactions SET status = $1 WHERE id = $2
	RETURNING status, created_at
	`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)
	if err := s.tx.QueryRowContext(ctx, query, status, tx.ID).Scan(&tx.Status, &tx.Created_at); err != nil {
		s.log.ErrorContext(ctx, "ошибка обновления статуса транзакции", "error", err, "transaction_id", tx.ID, "status", status)
		return fmt.Errorf("обновление статуса транзакции: %w", err)
	}
	s.log.InfoContext(ctx, "статус транзакции успешно обновлен", "transaction_id", tx.ID, "status", tx.Status)
	return nil
}

func (s *txRepo) GetByID(ctx context.Context, transactionID uuid.UUID) (domain.Transaction, error) {
	s.log.DebugContext(ctx, "получение транзакции по id", "transaction_id", transactionID)
	transaction := domain.Transaction{}
	query := `SELECT id, amount, sender_id, receiver_id, status, created_at FROM transactions WHERE id = $1`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)
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
		if errors.Is(err, sql.ErrNoRows) {
			s.log.WarnContext(ctx, "транзакция не найдена", "transaction_id", transactionID)
		} else {
			s.log.ErrorContext(ctx, "ошибка получения транзакции", "error", err, "transaction_id", transactionID)
		}
		return domain.Transaction{}, fmt.Errorf("получение транзакции по айди: %w", err)
	}
	s.log.DebugContext(ctx, "транзакция успешно получена", "transaction_id", transactionID, "status", transaction.Status)
	return transaction, nil
}

// GetTransactions получает транзакции по фильтрам
func (s *txRepo) GetTransactions(ctx context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	s.log.DebugContext(ctx, "получение транзакций по фильтрам", "filter", filter)

	query, args := sqlrequest(ctx, filter, s.log)
	if query == "" {
		s.log.Error("не получилось построить запрос")
		return nil, errors.New("не получилось построить запрос")
	}
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query, "args", args)

	rows, err := s.tx.QueryContext(ctx, query, args...)
	if err != nil {
		s.log.ErrorContext(ctx, "ошибка выполнения запроса", "error", err)
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
			s.log.ErrorContext(ctx, "ошибка сканирования строки", "error", err)
			return nil, fmt.Errorf("сканирование транзакции: %w", err)
		}
		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		s.log.ErrorContext(ctx, "ошибка при обработке строк", "error", err)
		return nil, fmt.Errorf("обработка строк результата: %w", err)
	}

	s.log.InfoContext(ctx, "транзакции успешно получены", "count", len(transactions))
	return transactions, nil
}

func (s *txRepo) TotalTransactions(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM transactions WHERE receiver_id=$1 OR sender_id=$1`
	var count int
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query)

	if err := s.tx.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (s *txToken) SaveRefreshToken(ctx context.Context, jti string, user_id string, expires_at time.Time) error {
	query := `INSERT INTO refresh_token(jti, user_id, expires_at, revoked) VALUES($1, $2, $3, false)`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query, "jti", jti, "user_id", user_id)

	res, err := s.tx.ExecContext(ctx, query, jti, user_id, expires_at)
	if err != nil {
		s.log.ErrorContext(ctx, "ошибка сохранения refresh токена", "err", err, "jti", jti)
		return domain.ErrSaveRefreshToken
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.log.ErrorContext(ctx, "проверка affected rows", "err", err)
		return err
	}

	if rows == 0 {
		s.log.WarnContext(ctx, "не удалось сохранить refresh токен - 0 rows affected", "jti", jti)
		return domain.ErrSaveRefreshToken
	}

	s.log.InfoContext(ctx, "refresh токен успешно сохранен", "jti", jti, "user_id", user_id)
	return nil
}

func (s *txToken) GetRefreshToken(ctx context.Context, jti string) (*domain.RefreshSession, error) {
	query := `SELECT user_id, revoked, expires_at FROM refresh_token WHERE jti = $1`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query, "jti", jti)

	session := &domain.RefreshSession{}
	err := s.tx.QueryRowContext(ctx, query, jti).Scan(&session.UserID, &session.Revoked, &session.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.log.WarnContext(ctx, "refresh токен не найден", "jti", jti)
			return nil, domain.ErrRefreshTokenNotFound
		}
		s.log.ErrorContext(ctx, "ошибка получения refresh токена", "err", err, "jti", jti)
		return nil, fmt.Errorf("получение refresh токена: %w", err)
	}

	s.log.DebugContext(ctx, "refresh токен успешно получен", "jti", jti, "user_id", session.UserID, "revoked", session.Revoked)
	return session, nil
}

func (s *txToken) RevokeRefreshToken(ctx context.Context, jti string) error {
	query := `UPDATE refresh_token SET revoked = true WHERE jti = $1 AND revoked = false`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query, "jti", jti)

	res, err := s.tx.ExecContext(ctx, query, jti)
	if err != nil {
		s.log.ErrorContext(ctx, "ошибка отзыва refresh токена", "err", err, "jti", jti)
		return fmt.Errorf("отзыв refresh токена: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.log.ErrorContext(ctx, "проверка affected rows при отзыве токена", "err", err)
		return err
	}

	if rows == 0 {
		s.log.WarnContext(ctx, "токен не найден или уже отозван", "jti", jti)
		return domain.ErrRefreshTokenNotFound
	}

	s.log.InfoContext(ctx, "refresh токен успешно отозван", "jti", jti)
	return nil
}

func (s *txToken) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE refresh_token SET revoked = true WHERE user_id = $1 AND revoked = false`
	s.log.DebugContext(ctx, "выполнение SQL запроса", "query", query, "user_id", userID)

	res, err := s.tx.ExecContext(ctx, query, userID)
	if err != nil {
		s.log.ErrorContext(ctx, "ошибка отзыва всех токенов пользователя", "err", err, "user_id", userID)
		return fmt.Errorf("отзыв всех токенов пользователя: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.log.ErrorContext(ctx, "проверка affected rows", "err", err)
		return err
	}

	s.log.InfoContext(ctx, "все токены пользователя отозваны", "user_id", userID, "revoked_count", rows)
	return nil
}
