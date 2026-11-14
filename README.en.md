# zhuzhao-utils

**[简体中文](README.md) | English**

**Shared Go infrastructure for the zhuzhao project family, imported package by package.**

Maintain signing, tokens, error responses, logging and connection setup in one place while keeping business data and authorization decisions in their owning services. Import a package without running zhuzhao.

Project family: [zhuzhao](https://github.com/tracerbiubiubiu/zhuzhao) · [zhuzhao-ui](https://github.com/tracerbiubiubiu/zhuzhao-ui) · [activelist](https://github.com/tracerbiubiubiu/activelist) · [taskrunner](https://github.com/tracerbiubiubiu/taskrunner) · **zhuzhao-utils**

## Packages

- **`aksk`:** internal HMAC-SHA256 signing/verification, an HTTP transport and Gin middleware. `aksk/compat` is a separate adapter for the legacy protocol.
- **`jwt`:** HS256 access/refresh tokens, token-type separation and password-epoch fields.
- **`crypto`:** bcrypt password hashing and verification.
- **`errcode`, `response`:** error-code primitives and Gin responses.
- **`logger`:** slog setup and log rotation.
- **`postgres`, `redis`:** pgxpool/go-redis constructors and a Redis login-failure lockout helper.
- **`jsonutil`, `validate`:** int64-array JSON encoding/decoding and identifier validation.

See the [package/API index](docs/api/README.md) and package sources for exact symbols and version boundaries. Actual dependencies follow the module and import graph.

## Run a signing round trip

Start in a new empty directory. This example pins **v0.4.3**, whose `go.mod` requires Go **1.26.0**. The current development branch has a separate minimum of Go **1.26.11** and a recommended toolchain of **1.27.4**; see [development](docs/guides/development.md).

```sh
go mod init example.com/aksk-demo
go get github.com/tracerbiubiubiu/zhuzhao-utils/aksk@v0.4.3
```

Save this complete program as `main.go`:

```go
package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/tracerbiubiubiu/zhuzhao-utils/aksk"
)

func main() {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	body := []byte(`{"action":"demo"}`)
	req, err := http.NewRequest(http.MethodPost,
		"https://example.invalid/tasks", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	aksk.Sign(req, body, aksk.SignOptions{
		AK: "demo-client", SK: key,
		Operator: "demo-user", RequestID: "readme-demo",
	})
	verifier := &aksk.Verifier{Keys: map[string][]byte{"demo-client": key}}
	if err := verifier.Verify(req, body); err != nil {
		panic(err)
	}
	fmt.Println("signature verified")
}
```

```sh
go run .
```

Expected output: `signature verified`. The key is generated in memory and never printed. This program sends no HTTP request and needs no database or Redis. Initial dependency installation needs module network access or a prepared cache. See [usage](docs/guides/usage.md) for real client/server wiring.

## Version and security boundaries

Pin a tag or immutable pseudo-version and retain `go.sum`. v0.4.3 is this example's explicit baseline, not a claim about the latest available version. Later Reader/Streaming APIs and audit fixes require a version that actually contains them; local HEAD does not establish availability in an older tag. See [release history](docs/ops/releases.md) and [version upgrades](docs/ops/version-upgrade.md).

Internal signatures cover method, path, body, timestamp, request ID and operator. **The query string is not signed, and the time window is not a single-use nonce.** Service authentication, operator attribution, business authorization and effect idempotency remain separate responsibilities; see the [AK/SK contract](docs/api/aksk.md). Consumers also own storage, increments and current-value checks for JWT password epochs.

This library contains shared mechanisms. Business tables, resource policies, model SDKs, capability catalogs and task state remain outside utils. The planned [platform mechanism contract](docs/api/platform-contracts.md) does not imply a new SDK or wire version has shipped.

## Documentation and contributing

- [Documentation](docs/README.md) · [Usage](docs/guides/usage.md) · [Packages/API](docs/api/README.md).
- [Development guide](docs/guides/development.md) · [Collaboration rules](AGENTS.md): define contracts and positive/negative cases first, then update implementations and check actual consumers.

```sh
make hooks
make lint
make test
make build
```

Here, `make lint` includes vet, formatting checks and tests. Describe version and consumer impact for public API changes. Contributions through [Issues](https://github.com/tracerbiubiubiu/zhuzhao-utils/issues) and focused PRs are welcome.

## License

[MIT](LICENSE).
