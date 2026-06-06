package transfer

//бизнес логика

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusCompleted TransactionStatus = "completed"
	StatusFailed    TransactionStatus = "failed"
)

type TransferService struct {
}
