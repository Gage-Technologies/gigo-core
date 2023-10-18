package external_api

import "testing"

func TestDetermineRoutePermission(t *testing.T) {
	tests := []struct {
		name string
		path string
		want RoutePermission
	}{
		{
			name: "private",
			path: "/api/workspace/status",
			want: RoutePermissionPrivate,
		},
		{
			name: "hybrid",
			path: "/api/search/users",
			want: RoutePermissionHybrid,
		},
		{
			name: "public",
			path: "/api/auth/loginWithGoogle",
			want: RoutePermissionPublic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetermineRoutePermission(tt.path); got != tt.want {
				t.Errorf("%s failed\n    Error %v != %v", t.Name(), got, tt.want)
			}
		})
		t.Logf("%s succeded", tt.name)
	}
}
