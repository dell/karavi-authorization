module karavi-authorization

go 1.16

require (
	github.com/alicebob/miniredis/v2 v2.14.3
	github.com/dell/goisilon v1.6.0
	github.com/dell/gopowermax v1.6.0
	github.com/dell/goscaleio v1.6.0
	github.com/dustin/go-humanize v1.0.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/golang/protobuf v1.4.3
	github.com/hashicorp/golang-lru v0.5.4
	github.com/julienschmidt/httprouter v1.3.0
	github.com/lestrrat-go/jwx v1.2.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.14.2 // indirect
	github.com/onsi/gomega v1.10.4 // indirect
	github.com/orlangure/gnomock v0.12.0
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.7.0
	github.com/valyala/fastjson v1.6.3
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.16.0
	go.opentelemetry.io/otel v0.16.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.16.0
	go.opentelemetry.io/otel/exporters/trace/zipkin v0.16.0
	go.opentelemetry.io/otel/sdk v0.16.0
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	google.golang.org/grpc v1.32.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	sigs.k8s.io/yaml v1.2.0
)
