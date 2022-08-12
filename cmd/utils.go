package cmd

import "os"

// Exit, limiting the code to a max of 125 (as recommended by os.Exit).
func exit(code int) {
	if code > 125 {
		code = 125
	}
	os.Exit(code)
}
