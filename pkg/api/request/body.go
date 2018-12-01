package request

import "github.com/golangci/golangci-api/internal/shared/logutil"

type Body []byte

func (b Body) FillLogContext(lctx logutil.Context) {
}
