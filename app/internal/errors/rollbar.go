package errors

import (
	"fmt"
	"os"

	"github.com/golangci/golangci-api/app/internal/auth/user"
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
	fields := []*rollbar.Field{}
	u, userErr := user.GetCurrent(ctx)
	if userErr != nil {
		fields = append(fields, &rollbar.Field{
			Name: "user",
			Data: u,
		})
	}

	if ctx.R != nil {
		go rollbar.RequestError(level, ctx.R, err, fields...)
	} else { // background
		go rollbar.Error(level, err, fields...)
	}

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
