package engine

import "testing"

func TestBuildGraphQLSubscriptionPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "normal path", in: "/graphql", want: "/graphql/ws"},
		{name: "trailing slash", in: "/graphql/", want: "/graphql/ws"},
		{name: "trim whitespace", in: " /graphql ", want: "/graphql/ws"},
		{name: "empty path", in: "", wantErr: true},
		{name: "whitespace only", in: "   ", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildGraphQLSubscriptionPath(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (path=%q)", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("path mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}
