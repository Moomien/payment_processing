package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// ─── Доменные сущности ───────────────────────────────────────────────────────

type User struct {
	ID    int
	Name  string
	Email string
}

type Order struct {
	ID     int
	UserID int
	Amount float64
}

// ─── Интерфейсы репозиториев ─────────────────────────────────────────────────

type UserRepository interface {
	Add(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id int) (*User, error)
}

type OrderRepository interface {
	Add(ctx context.Context, o *Order) error
}

// ─── Интерфейс Unit of Work ──────────────────────────────────────────────────

type UnitOfWork interface {
	Users() UserRepository
	Orders() OrderRepository
	Commit() error
	Rollback() error
}

// ─── Фабрика для создания UoW ────────────────────────────────────────────────

type UnitOfWorkFactory interface {
	NewUoW(ctx context.Context) (UnitOfWork, error)
}

// ─── SQL-реализация ──────────────────────────────────────────────────────────

// sqlUoW — одна транзакция, разделяемая двумя репозиториями
type sqlUoW struct {
	tx     *sql.Tx
	users  *sqlUserRepo
	orders *sqlOrderRepo
}

func (u *sqlUoW) Users() UserRepository   { return u.users }
func (u *sqlUoW) Orders() OrderRepository { return u.orders }
func (u *sqlUoW) Commit() error           { return u.tx.Commit() }
func (u *sqlUoW) Rollback() error         { return u.tx.Rollback() }

// sqlUoWFactory создаёт новую транзакцию и прокидывает её в оба репо
type sqlUoWFactory struct {
	db *sql.DB
}

func NewSQLUoWFactory(db *sql.DB) UnitOfWorkFactory {
	return &sqlUoWFactory{db: db}
}

func (f *sqlUoWFactory) NewUoW(ctx context.Context) (UnitOfWork, error) {
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	return &sqlUoW{
		tx:     tx,
		users:  &sqlUserRepo{tx: tx},
		orders: &sqlOrderRepo{tx: tx},
	}, nil
}

// ─── Репозитории (работают с *sql.Tx, не с *sql.DB) ─────────────────────────

type sqlUserRepo struct{ tx *sql.Tx }

func (r *sqlUserRepo) Add(ctx context.Context, u *User) error {
	return r.tx.QueryRowContext(ctx,
		`INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id`,
		u.Name, u.Email,
	).Scan(&u.ID)
}

func (r *sqlUserRepo) GetByID(ctx context.Context, id int) (*User, error) {
	u := &User{}
	err := r.tx.QueryRowContext(ctx,
		`SELECT id, name, email FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Name, &u.Email)
	return u, err
}

type sqlOrderRepo struct{ tx *sql.Tx }

func (r *sqlOrderRepo) Add(ctx context.Context, o *Order) error {
	return r.tx.QueryRowContext(ctx,
		`INSERT INTO orders (user_id, amount) VALUES ($1, $2) RETURNING id`,
		o.UserID, o.Amount,
	).Scan(&o.ID)
}

// ─── Сервисный слой — не знает ничего о SQL ──────────────────────────────────

type OrderService struct {
	uowFactory UnitOfWorkFactory
}

func NewOrderService(f UnitOfWorkFactory) *OrderService {
	return &OrderService{uowFactory: f}
}

// CreateUserWithOrder создаёт пользователя и заказ атомарно
func (s *OrderService) CreateUserWithOrder(ctx context.Context, name, email string, amount float64) error {
	uow, err := s.uowFactory.NewUoW(ctx)
	if err != nil {
		return err
	}

	// helper: откатить и обернуть ошибку
	fail := func(err error) error {
		_ = uow.Rollback()
		return err
	}

	user := &User{Name: name, Email: email}
	if err := uow.Users().Add(ctx, user); err != nil {
		return fail(fmt.Errorf("add user: %w", err))
	}

	order := &Order{UserID: user.ID, Amount: amount}
	if err := uow.Orders().Add(ctx, order); err != nil {
		return fail(fmt.Errorf("add order: %w", err))
	}

	if err := uow.Commit(); err != nil {
		return fail(fmt.Errorf("commit: %w", err))
	}

	fmt.Printf("✓ user=%d order=%d amount=%.2f\n", user.ID, order.ID, amount)
	return nil
}

// ─── Точка входа ─────────────────────────────────────────────────────────────

func main() {
	db, err := sql.Open("postgres", "postgres://user:pass@localhost/demo?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	factory := NewSQLUoWFactory(db)
	svc := NewOrderService(factory)

	if err := svc.CreateUserWithOrder(context.Background(), "Alice", "alice@example.com", 99.90); err != nil {
		log.Fatal(err)
	}
}
