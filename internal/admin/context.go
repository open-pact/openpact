package admin

import "context"

type contextKey string

const usernameKey contextKey = "username"

// WithUsername adds the username to the context.
func WithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, usernameKey, username)
}

// UsernameFromContext retrieves the username from the context.
func UsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(usernameKey).(string)
	return username, ok
}
