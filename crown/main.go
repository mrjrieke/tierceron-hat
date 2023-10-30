package main

import (
	"os"
	"strings"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)

// The original crown
func main() {
	exePath, exePathErr := os.Readlink("/proc/self/exe")
	if exePathErr != nil {
		os.Exit(-1)
	}
	brimPath := strings.Replace(exePath, "/crown", "/brim", 1)
	go tap.Tap(brimPath, "0904d372b7e10f44c7ea99b674d9ec19f7d2576a9d1e49c9530b37c45dd3eee6", "", false)

	tap.TapEyeRemember("eye", "rememeber")
	cap.TapMemorize("I think", "therefore I am.")
	cap.TapMemorize("It is not enough to have a good mind.", "The main thing is to use it well.")

	cap.TapServer("127.0.0.1:1534")
}
