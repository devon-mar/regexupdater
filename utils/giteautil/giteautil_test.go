package giteautil

import "testing"

func TestNextPage(t *testing.T) {
	tests := map[string]struct {
		linkHdr string
		want    int
	}{
		"first page": {
			linkHdr: `<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=2&state=all>; rel="next",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=20&state=all>; rel="last"`,
			want:    2,
		},
		"middle page": {
			linkHdr: `<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=3&state=all>; rel="next",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=20&state=all>; rel="last",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all>; rel="first",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all>; rel="prev"`,
			want:    3,
		},
		"last page": {
			linkHdr: `<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all>; rel="first",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=19&state=all>; rel="prev"`,
			want:    0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if have := NextPage(tc.linkHdr); have != tc.want {
				t.Errorf("got %d, want %d", have, tc.want)
			}
		})
	}
}
