module github.com/golangci/golangci-api

// +heroku goVersion go1.11
// +heroku install ./cmd/...

require (
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/ajg/form v1.5.1 // indirect
	github.com/aws/aws-lambda-go v1.11.1
	github.com/aws/aws-sdk-go v1.28.5
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/certifi/gocertifi v0.0.0-20190506164543-d2eda7129713 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.3.3 // indirect
	github.com/dukex/mixpanel v0.0.0-20180925151559-f8d5594f958e
	github.com/fatih/color v1.9.0
	github.com/fatih/structs v1.1.0 // indirect
	github.com/garyburd/redigo v1.6.0
	github.com/gavv/httpexpect v0.0.0-20170820080527-c44a6d7bb636
	github.com/gavv/monotime v0.0.0-20190418164738-30dba4353424 // indirect
	github.com/getsentry/raven-go v0.2.0
	github.com/go-kit/kit v0.9.0
	github.com/golang/mock v1.3.1
	github.com/golangci/golangci-lint v1.20.0
	github.com/gomodule/redigo v2.0.0+incompatible // indirect
	github.com/google/go-github v0.0.0-20180123235826-b1f138353a62
	github.com/gopherjs/gopherjs v0.0.0-20190430165422-3e4dfb77656c // indirect
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/schema v1.1.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.1.3
	github.com/imkira/go-interpol v1.1.0 // indirect
	github.com/jinzhu/gorm v1.9.12
	github.com/joho/godotenv v1.3.0
	github.com/k0kubun/colorstring v0.0.0-20150214042306-9440f1994b88 // indirect
	github.com/levigross/grequests v0.0.0-20190908174114-253788527a1a
	github.com/lib/pq v1.3.0
	github.com/markbates/goth v0.0.0-20180113214406-24f8ac10e57e
	github.com/mattes/migrate v0.0.0-20171208214826-d23f71b03c4a
	github.com/moul/http2curl v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/rs/cors v1.6.0
	github.com/satori/go.uuid v1.2.0
	github.com/savaki/amplitude-go v0.0.0-20160610055645-f62e3b57c0e4
	github.com/sergi/go-diff v1.0.0 // indirect
	github.com/shirou/gopsutil v0.0.0-20190901111213-e4ec7b275ada
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.5.1
	github.com/stvp/rollbar v0.5.1
	github.com/stvp/tempredis v0.0.0-20181119212430-b82af8480203 // indirect
	github.com/urfave/negroni v1.0.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	github.com/yalp/jsonpath v0.0.0-20180802001716-5cc68e5049a0 // indirect
	github.com/yudai/gojsondiff v1.0.0 // indirect
	github.com/yudai/golcs v0.0.0-20170316035057-ecda9a501e82 // indirect
	github.com/yudai/pp v2.0.1+incompatible // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/tools v0.0.0-20190930201159-7c411dea38b0
	gopkg.in/boj/redistore.v1 v1.0.0-20160128113310-fc113767cd6b
	gopkg.in/redsync.v1 v1.0.1
	gopkg.in/yaml.v2 v2.2.4
)

go 1.14
