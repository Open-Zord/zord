package bootstrap

import (
	"go-skeleton/pkg/config"
	"go-skeleton/pkg/database"
	"go-skeleton/pkg/idCreator"
	"go-skeleton/pkg/logger"
	"go-skeleton/pkg/registry"
	"go-skeleton/pkg/validator"
)

// registerPkg registers the primitive dependencies: logger, validator, config,
// idCreator and db (*sqlx.DB). Depends only on configs already loaded.
func registerPkg(reg *registry.Registry, conf *config.Config) {
	l := logger.NewLogger(
		conf.ReadConfig("ENVIRONMENT"),
		conf.ReadConfig("APP"),
		conf.ReadConfig("VERSION"),
	)
	l.Boot()

	db := database.NewMysql(
		l,
		conf.ReadConfig("DB_USER"),
		conf.ReadConfig("DB_PASS"),
		conf.ReadConfig("DB_URL"),
		conf.ReadConfig("DB_PORT"),
		conf.ReadConfig("DB_DATABASE"),
	)
	db.Connect()

	val := validator.NewValidator()
	val.Boot()

	idC := idCreator.NewIdCreator()

	reg.Provide(logger.RegistryKey, l)
	reg.Provide(validator.RegistryKey, val)
	reg.Provide(config.RegistryKey, conf)
	reg.Provide(idCreator.RegistryKey, idC)
	reg.Provide(database.RegistryKey, db.Db)
}
