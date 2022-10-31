package builder

import (
	"github.com/kong/go-kong/kong"
)

// KongsServiceBuilder is a builder for kong.Service.
// Primarily used for testing.
// Please note that some methods are not provided yet, as we
// don't need them yet. Feel free to add them as needed.
type KongsServiceBuilder struct {
	kongService kong.Service
}

func NewKongService(name string) *KongsServiceBuilder {
	return &KongsServiceBuilder{
		kongService: kong.Service{
			Name: kong.String(name),
		},
	}
}

func (b *KongsServiceBuilder) Build() kong.Service {
	return b.kongService
}

func (b *KongsServiceBuilder) WithHTTPDefaults() *KongsServiceBuilder {
	// This is bit of a hack, but it's the only way to get the default values
	// and use them in the parser tests without introducing a new packaged
	// with the default values.
	return b.WithProtocol("http").
		WithConnectTimeout(60000). // parser.DefaultServiceTimeout
		WithReadTimeout(60000).    // parser.DefaultServiceTimeout
		WithWriteTimeout(60000).   // parser.DefaultServiceTimeout
		WithRetries(5)             // parser.DefaultRetries
}

func (b *KongsServiceBuilder) WithProtocol(protocol string) *KongsServiceBuilder {
	b.kongService.Protocol = kong.String(protocol)
	return b
}

func (b *KongsServiceBuilder) WithHost(host string) *KongsServiceBuilder {
	b.kongService.Host = kong.String(host)
	return b
}

func (b *KongsServiceBuilder) WithPort(port int) *KongsServiceBuilder {
	b.kongService.Port = kong.Int(port)
	return b
}

func (b *KongsServiceBuilder) WithPath(path string) *KongsServiceBuilder {
	b.kongService.Path = kong.String(path)
	return b
}

func (b *KongsServiceBuilder) WithRetries(retries int) *KongsServiceBuilder {
	b.kongService.Retries = kong.Int(retries)
	return b
}

func (b *KongsServiceBuilder) WithConnectTimeout(connectTimeout int) *KongsServiceBuilder {
	b.kongService.ConnectTimeout = kong.Int(connectTimeout)
	return b
}

func (b *KongsServiceBuilder) WithWriteTimeout(writeTimeout int) *KongsServiceBuilder {
	b.kongService.WriteTimeout = kong.Int(writeTimeout)
	return b
}

func (b *KongsServiceBuilder) WithReadTimeout(readTimeout int) *KongsServiceBuilder {
	b.kongService.ReadTimeout = kong.Int(readTimeout)
	return b
}
