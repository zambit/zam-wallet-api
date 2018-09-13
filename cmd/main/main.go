package main

import (
	"git.zam.io/wallet-backend/wallet-api/cmd/listener"
	"git.zam.io/wallet-backend/wallet-api/cmd/root"
	"git.zam.io/wallet-backend/wallet-api/cmd/server"
	"git.zam.io/wallet-backend/wallet-api/cmd/watcher"
	"git.zam.io/wallet-backend/wallet-api/cmd/worker"
	"git.zam.io/wallet-backend/wallet-api/config"
	"github.com/spf13/viper"
)

// main executes specified command using cobra, on error will panic for nice stack print and non-zero exit code
func main() {
	var cfg config.RootScheme
	v := viper.New()

	config.Init(v)
	rootCmd := root.Create(v, &cfg)
	serverCmd := server.Create(v, &cfg)
	watcherCmd := watcher.Create(v, &cfg)
	listenerCmd := listener.Create(v, &cfg)
	workerCmd := worker.Create(v, &cfg)
	rootCmd.AddCommand(&serverCmd, &workerCmd, &listenerCmd, &watcherCmd)

	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
