package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/cmd/linter/analyzer"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
