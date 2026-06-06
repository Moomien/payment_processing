package storage

type Storage[T any] interface {
	Save(entity T) error
	Delete(id int) error
}
