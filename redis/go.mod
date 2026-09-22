module github.com/shanjunmei/go-session/redis

go 1.24

require (
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/redis/go-redis/v9 v9.22.0
	github.com/shanjunmei/go-session v0.1.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)

// During local development the parent module is resolved from the repository
// root so the nested module builds without a published parent tag.
replace github.com/shanjunmei/go-session => ../
