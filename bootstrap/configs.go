package bootstrap

import (
	"go-skeleton/pkg/config"
)

// loadConfigs reads the envs and returns the config instance plus the apiPrefix.
// apiPrefix comes out of config because it is consumed by the entrypoint to
// build the server; the rest of the envs are read on demand by the registrars
// in pkg.go through the *config.Config instance itself.
func loadConfigs() (conf *config.Config, apiPrefix string) {
	conf = config.NewConfig()
	if err := conf.LoadEnvs(); err != nil {
		panic(err)
	}
	apiPrefix = conf.ReadConfig("API_PREFIX")
	return conf, apiPrefix
}
