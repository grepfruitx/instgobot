module github.com/grepfruitx/instgobot

go 1.26.3

require (
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/caarlos0/env/v11 v11.4.1
	github.com/go-telegram/bot v1.22.0
	github.com/grepfruitx/snapmedia-downloader v0.0.0-00010101000000-000000000000
	github.com/pressly/goose/v3 v3.27.2
	github.com/redis/go-redis/v9 v9.21.0
	golang.org/x/sys v0.47.0
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	github.com/PuerkitoBio/goquery v1.12.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/gotd/td v0.161.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

replace github.com/grepfruitx/snapmedia-downloader => ../snapmedia-downloader
