package main

import (
	"log"

	"github.com/pp-sem7-team/backend/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
