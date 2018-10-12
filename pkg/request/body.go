package request

import "github.com/golangci/golangci-shared/pkg/logutil"

type Body []byte

func (b Body) FillLogContext(lctx logutil.Context) {
}
