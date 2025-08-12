module github.com/linkdata/rinse

go 1.23.2

toolchain go1.24.1

require (
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/linkdata/bytecount v1.2.0
	github.com/linkdata/deadlock v0.5.3
	github.com/linkdata/jaws v0.111.7
	github.com/linkdata/jawsauth v0.7.0
	github.com/linkdata/webserv v0.9.9
	github.com/swaggo/http-swagger v1.3.4
	github.com/swaggo/swag v1.16.6
	gitlab.com/jamietanna/content-negotiation-go v0.2.0
	golang.org/x/image v0.30.0
	golang.org/x/net v0.43.0
)

// replace github.com/linkdata/jawsauth => ../jawsauth
// replace github.com/linkdata/jaws => ../jaws

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/coder/websocket v1.8.13 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/petermattis/goid v0.0.0-20240813172612-4fcff4a6cae7 // indirect
	github.com/swaggo/files v1.0.1 // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sync v0.9.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/tools v0.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
