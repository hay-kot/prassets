package gitctx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRemoteURL(t *testing.T) {
	const prassets = "github.com/hay-kot/prassets"
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "git@github.com:hay-kot/prassets.git", want: prassets},
		{in: "git@github.com:Hay-Kot/PRAssets", want: prassets},
		{in: "https://github.com/hay-kot/prassets.git", want: prassets},
		{in: "https://github.com/hay-kot/prassets/", want: prassets},
		{in: "https://user:token@github.com/hay-kot/prassets.git", want: prassets}, //nolint:gosec // fake credentials in a URL fixture
		{in: "ssh://git@gitea.example.com:2222/hay-kot/prassets.git", want: "gitea.example.com/hay-kot/prassets"},
		{in: "https://gitlab.com/group/sub/proj.git", want: "gitlab.com/group/sub/proj"},
		{in: "/local/path/repo", wantErr: true},
		{in: "git@github.com:prassets.git", wantErr: true},
		{in: "https://github.com/a/../b", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseRemoteURL(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.ID())
		})
	}
}

func TestParseID(t *testing.T) {
	r, err := ParseID("GitHub.com/Acme/Widgets")
	require.NoError(t, err)
	assert.Equal(t, Repo{Host: "github.com", Owner: "acme", Name: "widgets"}, r)

	_, err = ParseID("acme/widgets")
	require.Error(t, err)
}
