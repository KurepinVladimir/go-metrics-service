package main

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/cmd/linter/analyzer"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer,
		"ok_main_only",
		"ok_misc",
		"bad_panic",
		"bad_fatal_outside_main",
		"bad_exit_in_lib",
	)
}
