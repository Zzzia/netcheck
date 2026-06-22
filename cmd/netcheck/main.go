package main

import (
	"log"
	"os"

	"github.com/Zzzia/netcheck/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
