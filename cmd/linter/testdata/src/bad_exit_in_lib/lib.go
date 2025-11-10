package lib

import "os"

func quit() {
	os.Exit(2) // want "os.Exit forbidden outside main.main"
}
