package main

import (
	"os"

	"github.com/zhirschtritt/eventing/internal/server"
)

func main() {
	if err := server.Execute(); err != nil {
		os.Exit(1)
	}
}
