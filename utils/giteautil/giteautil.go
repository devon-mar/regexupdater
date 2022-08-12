package giteautil

import (
	"net/url"
	"strconv"

	"github.com/devon-mar/regexupdater/utils/linkhdr"
)

func NextPage(linkHdr string) int {
	links := linkhdr.Parse(linkHdr)

	next := links["next"]
	if next == "" {
		return 0
	}

	parsed, err := url.Parse(next)
	if err != nil {
		return 0
	}
	page, _ := strconv.Atoi(parsed.Query().Get("page"))
	return page
}
