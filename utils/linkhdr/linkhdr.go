package linkhdr

import (
	"net/url"
	"strings"
)

func Parse(linkHdr string) map[string]string {
	ret := make(map[string]string, 4)
	if linkHdr == "" {
		return ret
	}

	// max 4 - prev, next, first, last
	split := strings.SplitN(linkHdr, ",", 4)
	for _, s := range split {
		linkPart, relPart, ok := strings.Cut(s, ";")
		if !ok {
			continue
		}
		linkPart = strings.TrimSpace(linkPart)
		relPart = strings.TrimSpace(relPart)

		if !strings.HasPrefix(relPart, `rel="`) || !strings.HasSuffix(relPart, `"`) || !strings.HasPrefix(linkPart, "<") || !strings.HasSuffix(linkPart, ">") {
			continue
		}

		rel := relPart[5 : len(relPart)-1]
		link := linkPart[1 : len(linkPart)-1]
		// try parsing the link to make sure it's valid
		if _, err := url.Parse(link); err != nil {
			continue
		}
		ret[rel] = link
	}
	return ret
}
