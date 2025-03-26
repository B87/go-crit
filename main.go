package main

import (
	"time"
	// Import SQL subcommand package
	// Import Zendesk subcommand package

	"github.com/spf13/viper"

	"github.com/b87/go-crit/cmd"
	_ "github.com/b87/go-crit/cmd/sql"
	_ "github.com/b87/go-crit/cmd/zendesk/sales"
)

func init() {
	// Set default timeout
	viper.SetDefault("timeout", 30*time.Second)
}

func main() {
	cmd.Execute()
}
