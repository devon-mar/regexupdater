package linkhdr

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := map[string]struct {
		in   string
		want map[string]string
	}{
		"invalid url": {
			in:   `<:::gitea.com/api/v1/repos/gitea/go-sdk/issues?page=2&state=all>; rel="next"`,
			want: map[string]string{},
		},
		"empty": {
			in:   "",
			want: map[string]string{},
		},
		"invalid url part": {
			in:   `<; rel="next"`,
			want: map[string]string{},
		},
		"no rel": {
			in:   `<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=2&state=all>`,
			want: map[string]string{},
		},
		"first page": {
			in: `<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=2&state=all>; rel="next",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=20&state=all>; rel="last"`,
			want: map[string]string{
				"next": "https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=2&state=all",
				"last": "https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=20&state=all",
			},
		},
		"middle page": {
			in: `<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=3&state=all>; rel="next",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=20&state=all>; rel="last",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all>; rel="first",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all>; rel="prev"`,
			want: map[string]string{
				"first": "https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all",
				"prev":  "https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all",
				"next":  "https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=3&state=all",
				"last":  "https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=20&state=all",
			},
		},
		"last page": {
			in: `<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all>; rel="first",<https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=19&state=all>; rel="prev"`,
			want: map[string]string{
				"first": "https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=1&state=all",
				"prev":  "https://gitea.com/api/v1/repos/gitea/go-sdk/issues?page=19&state=all",
			},
		},
		"docker registry": {
			in:   `</v2/library/alpine/tags/list?last=2.7&n=2>; rel="next"`,
			want: map[string]string{"next": "/v2/library/alpine/tags/list?last=2.7&n=2"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if have := Parse(tc.in); !reflect.DeepEqual(have, tc.want) {
				t.Errorf("got %#v, want %#v", have, tc.want)
			}
		})
	}
}
