// +build ignore

package main

import (
	"fmt"
	"log"

	"github.com/moto-nrw/project-phoenix/database"
)

func main() {
	db, err := database.DBConn()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()
	
	fmt.Println("Successfully connected to database!")
}