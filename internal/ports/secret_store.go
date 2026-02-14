package ports

import "context"

type SecretStore interface {
	Get(ctx context.Context, key string) (string, error)
	Put(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
}
