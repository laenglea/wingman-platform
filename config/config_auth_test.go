package config

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/auth"
)

func TestClientAuth(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *authConfig
		want    auth.TokenExchanger
		wantErr bool
	}{
		{
			name: "no auth",
		},
		{
			name: "static",
			cfg:  &authConfig{Type: "static", Token: "SERVICE"},
			want: auth.NewStaticExchanger("SERVICE"),
		},
		{
			name:    "static without token",
			cfg:     &authConfig{Type: "static"},
			wantErr: true,
		},
		{
			name: "passthrough",
			cfg:  &authConfig{Type: "passthrough"},
			want: auth.PassthroughExchanger{},
		},
		{
			name:    "unknown type",
			cfg:     &authConfig{Type: "nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createClientAuth(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if got != tt.want {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestClientAuthNoneIsNilInterface guards the typed-nil trap: returning a nil
// *obo.Exchanger as an interface would make the transport think auth is
// configured and strip the caller's credential.
func TestClientAuthNoneIsNilInterface(t *testing.T) {
	got, err := createClientAuth(nil)

	if err != nil {
		t.Fatal(err)
	}

	if got != nil {
		t.Errorf("got %#v, want a nil interface", got)
	}
}
