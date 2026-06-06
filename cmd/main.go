package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
)

func main() {
	id, err := uuid.NewUUID()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(id)
}
