package http

import (
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/config"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/database"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/idcreator"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/logger"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/registry"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/validator"
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

	db := database.NewMySQL(
		l,
		conf.ReadConfig("DB_USER"),
		conf.ReadConfig("DB_PASS"),
		conf.ReadConfig("DB_URL"),
		conf.ReadConfig("DB_PORT"),
		conf.ReadConfig("DB_DATABASE"),
	)
	if err := db.Connect(); err != nil {
		panic(err)
	}

	val := validator.NewValidator()
	val.Boot()

	idC := idcreator.NewIDCreator()

	reg.Provide(logger.RegistryKey, l)
	reg.Provide(validator.RegistryKey, val)
	reg.Provide(config.RegistryKey, conf)
	reg.Provide(idcreator.RegistryKey, idC)
	reg.Provide(database.RegistryKey, db.DB)
}
