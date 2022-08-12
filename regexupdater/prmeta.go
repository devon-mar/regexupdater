package regexupdater

import (
	"encoding/json"
	"strings"
)

const (
	lineCount = 3
)

type prMetadata struct {
	ID      string `json:"id"`
	Update  string `json:"update"`
	Version string `json:"version"`
}

func (m prMetadata) Footer() string {
	b, _ := json.Marshal(m)
	return "---\n`" + string(b) + "`"
}

func parsePRMeta(body string) prMetadata {
	prm := prMetadata{}
	split := strings.Split(body, "\n")
	var lastLines []string
	if len(split) < lineCount {
		lastLines = split
	} else {
		lastLines = split[len(split)-lineCount:]
	}
	for _, line := range lastLines {
		if strings.HasPrefix(line, "`") && strings.HasSuffix(line, "`") && len(line) > 2 {
			err := json.Unmarshal([]byte(line[1:len(line)-1]), &prm)
			if err == nil {
				return prm
			}
		}
	}
	return prm
}
