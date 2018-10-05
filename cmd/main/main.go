package main

import (
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/cmd/listener"
	"git.zam.io/wallet-backend/wallet-api/cmd/root"
	"git.zam.io/wallet-backend/wallet-api/cmd/server"
	"git.zam.io/wallet-backend/wallet-api/cmd/watcher"
	"git.zam.io/wallet-backend/wallet-api/cmd/worker"
	"git.zam.io/wallet-backend/wallet-api/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	commitSHA   = "undefined"
	commitRef   = "undefined"
	commitRep   = "undefined"
	commitEnv   = "undefined"
	commitPipID = "undefined"
)

// main executes specified command using cobra, on error will panic for nice stack print and non-zero exit code
func main() {
	logrus.Info("Server instance starting")
	var cfg config.RootScheme
	v := viper.New()

	config.Init(v)
	rootCmd := root.Create(v, &cfg)
	rootCmd.Version = fmt.Sprintf("%s-%s", commitRef, commitSHA)
	rootCmd.Long = fmt.Sprintf(
		`
Repository: %s
Built for env: %s
Built in pipeline: %s
		`,
		commitRep,
		commitEnv,
		commitPipID,
	)
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
