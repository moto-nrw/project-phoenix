//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/database"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{})

	db, err := database.DBConn()
	if err != nil {
		logger.WithError(err).Fatal("Error connecting to database")
	}
	defer db.Close()

	fmt.Println("Successfully connected to database!")
}
