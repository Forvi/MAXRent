// Package main Точка входа MAXRent бота.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Forvi/maxrent/internal/app"
	"github.com/Forvi/maxrent/internal/infrastructure/config"
)

// BuildTime Время сборки, проставляется через -ldflags "-X main.BuildTime=...".
var BuildTime = "unknown"

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fatal: %s\n", err)
		os.Exit(1)
	}
}

// run Собирает и запускает приложение, возвращая ошибку для обработки в main.
func run() error {
	cfg := config.LoadMust()

	application := app.BuildApp(context.Background(), cfg)

	return app.Run(context.Background(), application)
}
