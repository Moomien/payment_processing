package storage

import (
	"context"
	"database/sql"
	"log"
	"os"
	"processing/internal/decimal"
	transfer "processing/internal/service"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var testdb *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err :=
		postgres.Run(
			ctx, "postgres:16",
			postgres.WithDatabase("test_db"),
			postgres.WithUsername("testuser"),
			postgres.WithPassword("test-password"),
			postgres.BasicWaitStrategies())
	if err != nil {
		log.Fatal(err)
	}
	defer container.Terminate(ctx)

	connStr, err := container.ConnectionString(ctx, "sslmode=disable", "timezone=UTC")
	if err != nil {
		log.Fatal(err)
	}

	testdb, err = sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal(err)
	}

	if err = goose.Up(testdb, "../../migrations"); err != nil {
		log.Fatal(err)

	}
	os.Exit(m.Run())
}

func TestStorage(t *testing.T) {
	ctx := context.Background()
	tx, err := testdb.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	postgres := NewStorage(testdb, tx)
	// будет нашим sender_id
	balance, err := decimal.NewFromString("1100.11")
	if err != nil {
		t.Fatal(err)
	}
	alexAcc, err := NewAccount("alex", balance)
	if err != nil {
		t.Fatal(err)
	}

	if err = postgres.Create(ctx, alexAcc); err != nil {
		t.Fatal(err)
	}
	//receiver_id
	balance, err = decimal.NewFromString("390")
	if err != nil {
		t.Fatal(err)
	}
	testAcc, err := NewAccount("testacc", balance)
	if err != nil {
		t.Fatal(err)
	}

	if err = postgres.Create(ctx, testAcc); err != nil {
		t.Fatal(err)
	}

	acc, err := postgres.GetById(ctx, testAcc.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Успешно получен аккаунт")
	t.Log(acc)

	//пробуем создать транзакцию
	amount, err := decimal.NewFromString("100")
	if err != nil {
		t.Fatal(err)
	}

	tr, err := NewTransaction(amount, alexAcc.ID, testAcc.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := postgres.Transaction(ctx, tr); err != nil {
		t.Fatal(err)
	}

	if err := postgres.UpdateStatus(ctx, tr, transfer.StatusCompleted); err != nil {
		t.Fatal(err)
	}
	t.Log("создали транзакцию!")
	t.Log(tr)
}
