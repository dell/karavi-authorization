module karavi-authorization

go 1.15

require (
	github.com/dell/goscaleio v1.2.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.9.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-redis/redis/v8 v8.4.10
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.4.3
	github.com/hashicorp/golang-lru v0.5.1
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/rexray/gocsi v1.2.1
	github.com/orlangure/gnomock v0.12.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.6.1
	github.com/theckman/yacspin v0.8.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.16.0
	go.opentelemetry.io/otel v0.16.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.16.0
	go.opentelemetry.io/otel/exporters/trace/zipkin v0.16.0
	go.opentelemetry.io/otel/sdk v0.16.0
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	google.golang.org/grpc v1.32.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	sigs.k8s.io/kind v0.10.0 // indirect
	sigs.k8s.io/yaml v1.2.0
)
