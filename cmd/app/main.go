package main

import (
	"github.com/go-sdk/app"
	_ "github.com/go-sdk/database/dbx/postgres"

	_ "github.com/go-sdk/example/internal/migration"
	_ "github.com/go-sdk/example/internal/route"
	_ "github.com/go-sdk/example/internal/service"
)

func main() {
	app.Main()
}
