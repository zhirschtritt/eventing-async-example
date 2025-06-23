package main

import (
	"os"

	_ "github.com/outrigdev/outrig/autoinit"

	"github.com/zhirschtritt/eventing/internal/server"
)

func main() {
	if err := server.Execute(); err != nil {
		os.Exit(1)
	}
}
