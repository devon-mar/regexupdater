package regexupdater

import (
	"testing"
)

func TestParsePRMeta(t *testing.T) {
	tests := map[string]struct {
		in   string
		want prMetadata
	}{
		"not between quotes": {
			in: "{\"id\": \"123\",\"update\":\"u\",\"version\":\"1.2.0\"}\n",
		},
		"valid": {
			in: "\n`{\"id\": \"123\",\"update\":\"u\",\"version\":\"1.2.0\"}`\n",
			want: prMetadata{
				ID:      "123",
				Update:  "u",
				Version: "1.2.0",
			},
		},
		"between text": {
			in: "this is text\n`{\"id\": \"123\",\"update\":\"u\",\"version\":\"1.2.0\"}`\nother text",
			want: prMetadata{
				ID:      "123",
				Update:  "u",
				Version: "1.2.0",
			},
		},
		"not in last 3 lines": {
			in: "`{\"id\": \"123\",\"update\":\"u\",\"version\":\"1.2.0\"}`\n\n\n",
		},
		"empty": {
			in:   "",
			want: prMetadata{},
		},
		"invalid json": {
			in:   `{"id": "123"`,
			want: prMetadata{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			prMetaEqual(t, parsePRMeta(tc.in), tc.want)
		})
	}
}

func TestPRMetaFooter(t *testing.T) {
	tests := map[string]prMetadata{
		"all":          {ID: "123", Update: "test", Version: "abc"},
		"id only":      {ID: "123"},
		"version only": {Version: "abc"},
		"update only":  {Update: "test"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			prMetaEqual(t, tc, parsePRMeta(tc.Footer()))
		})
	}
}

func prMetaEqual(t *testing.T, have prMetadata, want prMetadata) {
	t.Helper()
	if have.ID != want.ID {
		t.Errorf("got ID %q, want %q", have.ID, want.ID)
	}
	if have.Update != want.Update {
		t.Errorf("got version %q, want %q", have.Update, want.Update)
	}
	if have.Version != want.Version {
		t.Errorf("got version %q, want %q", have.Version, want.Version)
	}
}
