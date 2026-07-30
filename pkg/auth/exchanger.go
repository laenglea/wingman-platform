package auth

import "context"

// TokenExchanger determines the credential sent to a downstream service.
//
// Token receives the caller's own token, which is empty when the caller is
// unauthenticated. An empty result sends no credential at all.
type TokenExchanger interface {
	Token(ctx context.Context, assertion string) (string, error)
}

// StaticExchanger sends a fixed credential, independent of the caller.
type StaticExchanger struct {
	token string
}

func NewStaticExchanger(token string) StaticExchanger {
	return StaticExchanger{token: token}
}

func (e StaticExchanger) Token(ctx context.Context, assertion string) (string, error) {
	return e.token, nil
}

// PassthroughExchanger sends the caller's own token to the downstream service
// unchanged.
//
// The MCP specification warns against token passthrough: the downstream server
// receives a token that was not issued for it, so it cannot validate the
// audience. Only use it when that server is trusted and shares the caller's
// identity provider; prefer an on-behalf-of exchange otherwise.
type PassthroughExchanger struct{}

func (PassthroughExchanger) Token(ctx context.Context, assertion string) (string, error) {
	return assertion, nil
}
