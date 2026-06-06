package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
)

const db_url = "postgres://admin:secret@localhost:5432/postgres_bd"

func main() {
	id, err := uuid.NewUUID()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(id)
}
