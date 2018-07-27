package root

import (
	"encoding/json"
	"git.zam.io/wallet-backend/wallet-api/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"strings"
)

// Create and initialize root command for given viper instance
func Create(v *viper.Viper, cfg *config.RootScheme) cobra.Command {
	var configPath string

	command := cobra.Command{
		Use: "wallet-api",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			if err = cmd.ParseFlags(args); err != nil {
				return
			}

			if configPath != "" {
				// Trying to open config
				v.SetConfigFile(configPath)

				// Attempts to load configuration
				err = v.ReadInConfig()
				if err != nil {
					return
				}
			}

			// allow env prefixes
			v.SetEnvPrefix("WA")
			v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
			v.AutomaticEnv()

			// Unmarshal it
			err = v.Unmarshal(cfg)
			if err != nil {
				return
			}

			e := json.NewEncoder(os.Stdout)
			e.SetIndent(" ", "\t")
			return e.Encode(cfg)
		},
	}

	command.Flags().StringVarP(
		&configPath, "config", "c", "", "specifies configuration file to load from",
	)
	command.Flags().StringP(
		"env", "e", "test", "specifies current environment (prod/dev/test)",
	)
	v.BindPFlags(command.Flags())

	return command
}
