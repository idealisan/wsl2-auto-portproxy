module github.com/HobaiRiku/wsl2-auto-portproxy

go 1.27.3

require github.com/egoist/mygo v0.8.3

// Pin the CLI used by `go tool mygo build` to package the macOS app.
tool github.com/egoist/mygo/cmd/mygo

require (
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)
