module github.com/golangci/golangci-api

// +heroku goVersion go1.11
// +heroku install ./cmd/...

require (
	cloud.google.com/go v0.33.1 // indirect
	github.com/Microsoft/go-winio v0.4.11 // indirect
	github.com/ajg/form v0.0.0-20160822230020-523a5da1a92f // indirect
	github.com/aws/aws-lambda-go v1.6.0
	github.com/aws/aws-sdk-go v0.0.0-20180126231901-00cca3f093a8
	github.com/cenkalti/backoff v2.0.0+incompatible
	github.com/certifi/gocertifi v0.0.0-20180905225744-ee1a9a0726d2 // indirect
	github.com/denisenkom/go-mssqldb v0.0.0-20181014144952-4e0d7dc8888f // indirect
	github.com/docker/distribution v2.6.2+incompatible // indirect
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.3.3 // indirect
	github.com/dukex/mixpanel v0.0.0-20170510165255-53bfdf679eec
	github.com/erikstmartin/go-testdb v0.0.0-20160219214506-8d10e4a1bae5 // indirect
	github.com/fatih/color v1.7.0
	github.com/fatih/structs v1.1.0 // indirect
	github.com/garyburd/redigo v1.5.0
	github.com/gavv/httpexpect v0.0.0-20170820080527-c44a6d7bb636
	github.com/gavv/monotime v0.0.0-20171021193802-6f8212e8d10d // indirect
	github.com/getsentry/raven-go v0.0.0-20180801005657-7535a8fa2ace
	github.com/go-ini/ini v1.39.0 // indirect
	github.com/go-kit/kit v0.7.0
	github.com/go-logfmt/logfmt v0.4.0 // indirect
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/golang/mock v1.2.0
	github.com/golangci/golangci-lint v1.17.0
	github.com/gomodule/redigo v2.0.0+incompatible // indirect
	github.com/google/go-github v0.0.0-20180123235826-b1f138353a62
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v0.0.0-20180120075819-c0091a029979
	github.com/gorilla/schema v1.0.2
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v0.0.0-20180115173807-fe21b6a095cd
	github.com/imkira/go-interpol v1.1.0 // indirect
	github.com/jinzhu/gorm v1.9.1
	github.com/jinzhu/inflection v0.0.0-20180308033659-04140366298a // indirect
	github.com/jinzhu/now v0.0.0-20181116074157-8ec929ed50c3 // indirect
	github.com/jmespath/go-jmespath v0.0.0-20180206201540-c2b33e8439af // indirect
	github.com/joho/godotenv v0.0.0-20180115024921-6bb08516677f
	github.com/jtolds/gls v4.2.1+incompatible // indirect
	github.com/k0kubun/colorstring v0.0.0-20150214042306-9440f1994b88 // indirect
	github.com/levigross/grequests v0.0.0-20180717012718-3f841d606c5a
	github.com/lib/pq v0.0.0-20180201184707-88edab080323
	github.com/markbates/goth v0.0.0-20180113214406-24f8ac10e57e
	github.com/mattes/migrate v0.0.0-20171208214826-d23f71b03c4a
	github.com/mattn/go-sqlite3 v1.10.0 // indirect
	github.com/moul/http2curl v1.0.0 // indirect
	github.com/pkg/errors v0.8.0
	github.com/rs/cors v0.0.0-20170801073201-eabcc6af4bbe
	github.com/satori/go.uuid v1.2.0
	github.com/savaki/amplitude-go v0.0.0-20160610055645-f62e3b57c0e4
	github.com/sergi/go-diff v1.0.0 // indirect
	github.com/shirou/gopsutil v0.0.0-20180801053943-8048a2e9c577
	github.com/sirupsen/logrus v1.0.5
	github.com/smartystreets/assertions v0.0.0-20180927180507-b2de0cb4f26d // indirect
	github.com/smartystreets/goconvey v0.0.0-20181108003508-044398e4856c // indirect
	github.com/spf13/viper v1.0.2
	github.com/stevvooe/resumable v0.0.0-20180830230917-22b14a53ba50 // indirect
	github.com/stretchr/testify v1.2.2
	github.com/stvp/rollbar v0.5.1
	github.com/stvp/tempredis v0.0.0-20181119212430-b82af8480203 // indirect
	github.com/urfave/negroni v0.0.0-20180105164225-ff85fb036d90
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20181112162635-ac52e6811b56 // indirect
	github.com/yalp/jsonpath v0.0.0-20180802001716-5cc68e5049a0 // indirect
	github.com/yudai/gojsondiff v1.0.0 // indirect
	github.com/yudai/golcs v0.0.0-20170316035057-ecda9a501e82 // indirect
	github.com/yudai/pp v2.0.1+incompatible // indirect
	golang.org/x/oauth2 v0.0.0-20180118004544-b28fcf2b08a1
	golang.org/x/sync v0.0.0-20181108010431-42b317875d0f // indirect
	golang.org/x/tools v0.0.0-20190420000508-685fecacd0a0
	google.golang.org/appengine v1.3.0 // indirect
	gopkg.in/boj/redistore.v1 v1.0.0-20160128113310-fc113767cd6b
	gopkg.in/ini.v1 v1.39.0 // indirect
	gopkg.in/redsync.v1 v1.0.1
	gopkg.in/yaml.v2 v2.2.1
)
