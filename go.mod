module yatori-go-console

go 1.24.11

// 若出现hash校验错误请使用以下指令
// set GONOSUMDB=*
// set GOPROXY=direct
//然后自行设置代理，set http_proxy=http:localhost:xxxx 你的VPN本地代理
//然后自行设置代理，set https_proxy=http:localhost:xxxx 你的VPN本地代理
require (
	//github.com/faiface/beep v1.1.0
	github.com/spf13/viper v1.21.0
	github.com/thedevsaddam/gojsonq v2.3.0+incompatible
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
)

require (
	github.com/gofrs/flock v0.13.0
	github.com/yatori-dev/yatori-go-core v1.9.1-0.20260724131310-c9cddbeb4e4c //v2.0.5.0
	golang.org/x/sys v0.39.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Changbaiqi/ddddocr-go v0.1.5-0.20251217103242-1896343369f8 // indirect
	github.com/PuerkitoBio/goquery v1.10.2 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/yalue/onnxruntime_go v1.24.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/image v0.34.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
)
