package main

import (
	"log"

	"github.com/pp-sem7-team/backend/internal/app"
)

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
