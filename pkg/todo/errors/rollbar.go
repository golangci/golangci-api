package errors

import (
	"fmt"
	"os"

	"github.com/golangci/golangci-api/pkg/apperrors"
	"github.com/golangci/golangci-api/pkg/config"
	"github.com/golangci/golangci-api/pkg/logutil"

	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	"github.com/golangci/golib/server/context"
	"github.com/stvp/rollbar"
)

func Warnf(ctx *context.C, format string, args ...interface{}) {
	track(ctx, fmt.Errorf(format, args...), "WARN")
}

func Error(ctx *context.C, err error) {
	track(ctx, err, "ERROR")
}

func Errorf(ctx *context.C, format string, args ...interface{}) {
	track(ctx, fmt.Errorf(format, args...), "ERROR")
}

func track(ctx *context.C, err error, level string) {
	log := logutil.NewStderrLog("track")
	cfg := config.NewEnvConfig(log)
	et := GetTracker(cfg, log)
	if ctx.R != nil {
		et = et.WithHTTPRequest(ctx.R)
	}

	ectx := map[string]interface{}{}

	u, _ := user.GetCurrent(ctx)
	if u != nil {
		ectx["userID"] = u.ID
		ectx["email"] = u.Email
	}

	et.Track(apperrors.Level(level), err.Error(), ectx)

	if level == "ERROR" {
		ctx.L.Errorf("%s: %+v", err, u)
	} else {
		ctx.L.Warnf("%s: %+v", err, u)
	}
}

func init() {
	rollbar.Token = os.Getenv("ROLLBAR_API_TOKEN")
	goEnv := os.Getenv("GO_ENV")
	if goEnv == "prod" {
		rollbar.Environment = "production"
	} else {
		rollbar.Environment = "development"
	}
}
