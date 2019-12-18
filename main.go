package main

import (
    "path/filepath"
    "github.com/sirkon/goproxy/gomod"
)

const goModFilename = "go.mod"

const workspace = os.Getenv("GITHUB_WORKSPACE")
const token = os.Getenv("GITHUB_TOKEN")
const inReportPath = os.Getenv("INPUT_REPORT_PATH")

// NOTE: https://github.com/golang/vgo/blob/master/vendor/cmd/internal/test2json/test2json.go#L31
type event struct {
	Time    *time.Time `json:",omitempty"`
	Action  string
	Package string     `json:",omitempty"`
	Test    string     `json:",omitempty"`
	Elapsed *float64   `json:",omitempty"`
	Output  *textBytes `json:",omitempty"`
}

type report struct {

}

func main() {
    rootDir := workspace
    goModPath := filepath.Join(workspace, goModFilename)
    reportPath := filepath.JOIN(workspace, inReportPath)

    modName, err := readModName(goModPath)
    if err != nil {
        log.Fatalf("failed to read module name: %v", err)
    }

    reports, err := readReporter(reportPath)
    if err != nil {
        log.Fatalf("failed to read report file: %v", err)
    }

}

func readModName(goModFilename string) (string, error) {
    buf, err := ioutil.ReadFile(goModFilename)
    if err != nil {
        return "", fmt.Errorf("cannot read go.mod file: %w", err)
    }

    mod, err := gomod.Parse(goModPath, buf)
    if err != nil {
        return "", fmt.Errorf("parse error: %w", err)
    }

    return mod.Name, nil
}

func readReporter(reportFilename string) ([]report, error) {
    buf, err := ioutil.ReadFile(reportFilename)
    if err != nil {
        return nil, fmt.Errorf("cannot read go.mod file: %w", err)
    }

    return []report{}, nil
}