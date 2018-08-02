package server

import (
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/dig"
	"io"
	"time"

	"context"
	"git.zam.io/wallet-backend/wallet-api/config"
	serverconf "git.zam.io/wallet-backend/wallet-api/config/server"
	walletconf "git.zam.io/wallet-backend/wallet-api/config/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/wallets"
	wmiddlewares "git.zam.io/wallet-backend/wallet-api/internal/server/middlewares"
	swallets "git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	_ "git.zam.io/wallet-backend/wallet-api/internal/services/nodes/btc"
	"git.zam.io/wallet-backend/web-api/db"
	_ "git.zam.io/wallet-backend/web-api/server/handlers"
	"git.zam.io/wallet-backend/web-api/server/handlers/base"
	"git.zam.io/wallet-backend/web-api/server/handlers/static"
	"git.zam.io/wallet-backend/web-api/server/middlewares"
	"git.zam.io/wallet-backend/web-api/services/nosql"
	nosqlfactory "git.zam.io/wallet-backend/web-api/services/nosql/factory"
	"git.zam.io/wallet-backend/web-api/services/sessions"
	sessjwt "git.zam.io/wallet-backend/web-api/services/sessions/jwt"
	sessmem "git.zam.io/wallet-backend/web-api/services/sessions/mem"
	"github.com/pkg/errors"
)

// Create and initialize server command for given viper instance
func Create(v *viper.Viper, cfg *config.RootScheme) cobra.Command {
	command := cobra.Command{
		Use:   "server",
		Short: "Runs Wallet-API server",
		RunE: func(_ *cobra.Command, args []string) error {
			return serverMain(*cfg)
		},
	}
	// add common flags
	command.Flags().StringP("server.host", "l", v.GetString("server.host"), "host to serve on")
	command.Flags().IntP("server.port", "p", v.GetInt("server.port"), "port to serve on")
	command.Flags().String(
		"db.uri",
		v.GetString("db.uri"),
		"postgres connection uri",
	)
	v.BindPFlags(command.Flags())

	return command
}

// serverMain
func serverMain(cfg config.RootScheme) (err error) {
	// create DI container and populate it with providers
	c := dig.New()

	// provide root logger
	rootLogger := logrus.New()
	err = c.Provide(func() logrus.FieldLogger {
		return rootLogger
	})
	if err != nil {
		return
	}

	// provide ordinal db connection
	err = c.Provide(db.Factory(cfg.DB.URI))
	if err != nil {
		return
	}

	// provide sessions storage
	err = c.Provide(func(conf serverconf.Scheme, persistentStorage nosql.IStorage) (res sessions.IStorage, err error) {
		// catch jwt storage panics
		defer func() {
			r := recover()
			if r != nil {
				if e, ok := r.(error); ok {
					err = e
				} else {
					panic(r)
				}
			}
		}()

		switch conf.Auth.TokenStorage {
		case "mem", "":
			return sessmem.New(), nil
		case "jwt", "jwtpersistent":
			if conf.JWT == nil {
				return nil, errors.New("jwt like token storage required, but jwt configuration not provided")
			}
			res = sessjwt.New(conf.JWT.Method, []byte(conf.JWT.Secret), func() time.Time { return time.Now().UTC() })

			if conf.Auth.TokenStorage == "jwtpersistent" {
				res = sessjwt.WithStorage(
					res, persistentStorage, func(data map[string]interface{}, token string) string {
						return fmt.Sprintf("user:%v:sessions", data["phone"])
					},
				)
			}
			return
		default:
			return nil, fmt.Errorf("unsupported token storage type: %s", conf.Auth.TokenStorage)
		}
	})
	if err != nil {
		return
	}

	// provide nosql storage
	err = c.Provide(func(conf serverconf.Scheme) (nosql.IStorage, io.Closer, error) {
		if conf.Storage.URI == "" {
			conf.Storage.URI = "mem://"
		}
		return nosqlfactory.NewFromUri(conf.Storage.URI)
	})
	if err != nil {
		return
	}

	err = c.Provide(func() (serverconf.Scheme, walletconf.Scheme) {
		return cfg.Server, cfg.Wallets
	})
	if err != nil {
		return
	}

	// provide gin engine
	err = c.Provide(func(logger logrus.FieldLogger) *gin.Engine {
		corsCfg := cors.DefaultConfig()
		corsCfg.AllowMethods = append(corsCfg.AllowMethods, "DELETE")
		corsCfg.AllowAllOrigins = true
		corsCfg.AllowHeaders = []string{"*"}

		engine := gin.New()
		engine.Use(
			gin.Recovery(),
			gin.Logger(),
			cors.New(corsCfg),
		)
		return engine
	})
	if err != nil {
		return
	}

	// provide api router
	err = c.Provide(func(engine *gin.Engine) gin.IRouter {
		return engine.Group("/api/v1")
	}, dig.Name("api_routes"))
	if err != nil {
		return
	}

	// provide wallets generator
	err = c.Provide(func(wConf walletconf.Scheme, logger logrus.FieldLogger) (coordinator swallets.ICoordinator, err error) {
		coordinator = swallets.New(logger)
		for coinName, nodeConf := range wConf.CryptoNodes {
			rootLogger.WithField("conn_params", nodeConf).Infof("connecting %s node", coinName)
			err = coordinator.Dial(coinName, nodeConf.Host, nodeConf.User, nodeConf.Pass, nodeConf.Testnet)
			if err != nil {
				rootLogger.WithError(err).Errorf("connecting node %s has been failed", coinName)
				return
			}
		}
		return
	})
	if err != nil {
		return
	}

	// provide auth middleware
	err = c.Provide(func(sessStorage sessions.IStorage) gin.HandlerFunc {
		return middlewares.AuthMiddlewareFactory(sessStorage, cfg.Server.Auth.TokenName)
	}, dig.Name("auth_middleware"))
	if err != nil {
		return
	}

	// provide user middleware
	err = c.Provide(func(sessStorage sessions.IStorage) gin.HandlerFunc {
		return base.WrapMiddleware(wmiddlewares.UserMiddlewareFactory(
			func(c context.Context) (userID int64, present bool, valid bool) {
				user := middlewares.GetUserDataFromContext(c.(*gin.Context))

				rawID, present := user["id"]
				if !present {
					return
				}
				fID, valid := rawID.(float64)
				if !valid {
					return
				}
				userID = int64(fID)
				return
			},
		))
	}, dig.Name("user_middleware"))
	if err != nil {
		return
	}

	// close all resources
	defer c.Invoke(func(coordinator swallets.ICoordinator, closers ...io.Closer) {
		for _, c := range closers {
			c.Close()
		}

		err := coordinator.Close()
		if err != nil {
			rootLogger.WithError(err).Error("error occurred while closing wallets coordinator")
		}
	})

	// Run server!
	err = c.Invoke(func(engine *gin.Engine, dependencies wallets.Dependencies) error {
		wallets.Register(c)
		static.Register(engine)
		return engine.Run(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
	})

	return
}
