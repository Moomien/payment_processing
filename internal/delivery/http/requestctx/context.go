package requestctx

import "context"

// Identity содержит данные аутентифицированного пользователя из HTTP-запроса.
type Identity struct {
	UserID string
	Role   string
}

type identityKey struct{}

// WithIdentity возвращает копию ctx с данными аутентифицированного пользователя.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}

// IdentityFrom возвращает данные аутентифицированного пользователя из ctx.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok
}
