package sharedtest

import (
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/test/sharedtest/mocks"
	"github.com/jinzhu/gorm"
)

type CommonDeps struct {
	DB              *gorm.DB
	Cfg             config.Config
	Log             logutil.Log
	ProviderFactory providers.Factory
}

func (ta *App) BuildCommonDeps() *CommonDeps {
	log := logutil.NewStderrLog("test")
	cfg := config.NewEnvConfig(log)

	dbConnString, err := gormdb.GetDBConnString(cfg)
	if err != nil {
		log.Fatalf("Can't get DB conn string: %s", err)
	}

	gormDB, err := gormdb.GetDB(cfg, log, dbConnString)
	if err != nil {
		log.Fatalf("Can't get gorm db: %s", err)
	}

	origPF := providers.NewBasicFactory(log)
	pf := mocks.NewProviderFactory(func(p provider.Provider) provider.Provider {
		if err := p.SetBaseURL(ta.fakeGithubServer.URL + "/"); err != nil {
			log.Fatalf("Failed to set base url: %s", err)
		}
		return p
	}, origPF)

	return &CommonDeps{
		DB:              gormDB,
		Cfg:             cfg,
		Log:             log,
		ProviderFactory: pf,
	}
}
