package buildinfo

import "fmt"

var (
	Version string
	Date    string
	Commit  string
)

func Print() {
	fmt.Printf("Build version: %s\n", valueOrNA(Version))
	fmt.Printf("Build date: %s\n", valueOrNA(Date))
	fmt.Printf("Build commit: %s\n", valueOrNA(Commit))
}

func valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}
