package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirkon/goproxy/gomod"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sync/errgroup"
)

func getArgs() []string {
	return os.Args[1:]
}

func getSubcommand() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return ""
}

func readLine(r *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		b, isPrefix, err := r.ReadLine()
		if err != nil {
			return "", err
		}

		sb.Write(b)
		if !isPrefix {
			break
		}
	}
	return sb.String(), nil
}

type annotation struct {
	file string
	line int
	col  int
	text string
}

type buildErr struct {
	file string
	line int
	col  int
	text string
}

func (b buildErr) output() string {
	file := strings.TrimPrefix(b.file, "./")
	return fmt.Sprintf("::error file=%s,line=%d,col=%d:: %s", file, b.line, b.col, b.text)
}

func matchBuildErr(str string) *buildErr {
	if m := reBuildErr.FindStringSubmatch(str); len(m) > 0 {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		return &buildErr{
			file: m[1],
			line: line,
			col:  col,
			text: m[4],
		}
	}
	return nil
}

type testErr struct {
	pkg  string
	file string
	line int
	text string
}

func matchTestErr(str string) *testErr {
	if m := reTestErr.FindStringSubmatch(str); len(m) > 0 {
		line, _ := strconv.Atoi(m[2])
		return &testErr{
			file: m[1],
			line: line,
			text: m[3],
		}
	}
	return nil
}

var (
	reBuildErr      = regexp.MustCompile(`^(.+\.go):(\d+):(\d+):\s(.*)$`)
	reTestStartErr  = regexp.MustCompile(`^---\sFAIL:\s+(.+)\s+\([0-9\.]+[smh]\)$`)
	reTestErr       = regexp.MustCompile(`^\s+(.+\.go):(\d+):\s(.+)$`)
	reTestResultErr = regexp.MustCompile(`^FAIL\s(.+)\s+[0-9\.]+[smh]$`)
)

func createBuildErr(eg *errgroup.Group, out *os.File) io.Writer {
	r, w := io.Pipe()
	cmdOutReader := bufio.NewReader(r)
	eg.Go(func() error {
		for {
			line, err := readLine(cmdOutReader)
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("cannot read line: %w", err)
			}

			if m := matchBuildErr(line); m != nil {
				fmt.Fprintf(out, "%s\n", m.output())
			}

			fmt.Fprintf(out, "%s\n", line)
		}
		return nil
	})
	return w
}

type testErrBuilder struct {
	pkgMap    map[string]string
	workspace string
}

func createTestErrBuilder(args []string) testErrBuilder {
	workspace, _ := os.Getwd()
	globs := parseTestFlag(args)
	pkgMap := createPkgMap(globs)
	return testErrBuilder{
		pkgMap:    pkgMap,
		workspace: workspace,
	}
}

func (t testErrBuilder) createTestErr(eg *errgroup.Group, out *os.File) io.Writer {
	r, w := io.Pipe()
	cmdOutReader := bufio.NewReader(r)
	eg.Go(func() error {
		var testErrs []testErr
		for {
			line, err := readLine(cmdOutReader)
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("cannot read line: %w", err)
			}

			if testErrs == nil {
				if reTestStartErr.MatchString(line) {
					testErrs = make([]testErr, 0)
				}
			} else {
				if m := matchTestErr(line); m != nil {
					testErrs = append(testErrs, *m)
				}
				if m := reTestResultErr.FindStringSubmatch(line); len(m) > 0 {
					pkg := m[1]
					for i := range testErrs {
						fmt.Fprintf(out, "%s\n", t.outputErr(testErrs[i], pkg))
					}
					testErrs = nil
				}
			}

			fmt.Fprintf(out, "%s\n", line)
		}
		return nil
	})
	return w
}

func (t testErrBuilder) outputErr(te testErr, pkg string) string {
	dir := t.getFilepathByPkg(pkg)
	file, _ := filepath.Rel(t.workspace, filepath.Join(dir, te.file))
	file = strings.TrimPrefix(file, "./")
	return fmt.Sprintf("::error file=%s,line=%d:: %s", file, te.line, te.text)
}

func (t testErrBuilder) getFilepathByPkg(pkg string) string {
	return t.pkgMap[pkg]
}

// Get as many globs as possible.
func parseTestFlag(args []string) []string {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.SetOutput(ioutil.Discard)
	// https://github.com/golang/go/blob/master/src/cmd/go/internal/test/testflag.go
	// HACK: This may not be fully compatible.
	fs.Bool("c", false, "")
	fs.Bool("i", false, "")
	fs.String("o", "", "")
	fs.Bool("cover", false, "")
	fs.String("covermode", "", "")
	fs.String("coverpkg", "", "")
	fs.String("exec", "", "")
	fs.Bool("json", false, "")
	fs.String("vet", "", "")
	fs.String("bench", "", "")
	fs.Bool("benchmem", false, "")
	fs.String("benchtime", "", "")
	fs.String("blockprofile", "", "")
	fs.String("blockprofilerate", "", "")
	fs.String("count", "", "")
	fs.String("coverprofile", "", "")
	fs.String("cpu", "", "")
	fs.String("cpuprofile", "", "")
	fs.Bool("failfast", false, "")
	fs.String("list", "", "")
	fs.String("memprofile", "", "")
	fs.String("memprofilerate", "", "")
	fs.String("mutexprofile", "", "")
	fs.String("mutexprofilefraction", "", "")
	fs.String("outputdir", "", "")
	fs.String("parallel", "", "")
	fs.String("run", "", "")
	fs.Bool("short", false, "")
	fs.String("timeout", "", "")
	fs.String("trace", "", "")
	fs.Bool("v", false, "")
	fs.Parse(args[1:])

	paths := fs.Args()
	return paths
}

func createPkgMap(paths []string) map[string]string {
	dirMemo := map[string]string{}

	var detectPackage func(string) string
	detectPackage = func(dir string) string {
		if pkg, ok := dirMemo[dir]; ok {
			return pkg
		}
		base := filepath.Base(dir)
		parent := filepath.Dir(dir)
		gomodPath := filepath.Join(dir, "go.mod")

		fi, err := os.Stat(gomodPath)
		if err != nil || fi.IsDir() {
			goto findNext
		}
		{
			b, err := ioutil.ReadFile(gomodPath)
			if err != nil {
				goto findNext
			}
			mod, err := gomod.Parse(gomodPath, b)
			if err != nil {
				goto findNext
			}
			dirMemo[dir] = mod.Name
			return mod.Name
		}
	findNext:
		if dir == "/" {
			dirMemo[dir] = ""
			return ""
		}
		parentPkgName := detectPackage(parent)
		if parentPkgName == "" {
			dirMemo[dir] = ""
			return ""
		}
		myPkgName := path.Join(parentPkgName, base)
		dirMemo[dir] = myPkgName
		return myPkgName
	}

	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			log.Printf("[warning] Detect package: %v", err)
			continue
		}
		base := filepath.Base(abs)
		dir := filepath.Dir(abs)
		if base == "..." {
			filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				detectPackage(path)
				return nil
			})
		} else {
			stat, err := os.Stat(abs)
			if err != nil {
				continue
			}
			if stat.IsDir() {
				detectPackage(abs)
			} else {
				detectPackage(dir)
			}
		}
	}

	pkgMap := map[string]string{}
	for dir, pkg := range dirMemo {
		pkgMap[pkg] = dir
	}
	return pkgMap
}

func injectStdout(cmd *exec.Cmd, eg *errgroup.Group, subCmd string, args []string) {
	switch subCmd {
	case "build", "run":
		cmd.Stdout = os.Stdout
		cmd.Stderr = createBuildErr(eg, os.Stderr)
	case "test":
		testBuilder := createTestErrBuilder(args)
		cmd.Stdout = testBuilder.createTestErr(eg, os.Stdout)
		cmd.Stderr = createBuildErr(eg, os.Stderr)
	default:
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
}

func main() {
	isPipe := !terminal.IsTerminal(0)
	args := getArgs()
	subCmd := getSubcommand()
	cmd := exec.Command("go", args...)

	if isPipe {
		cmd.Stdin = os.Stdin
	}

	eg, _ := errgroup.WithContext(context.Background())
	injectStdout(cmd, eg, subCmd, args)

	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to start go command: %v", err)
	}

	eg.Go(func() error {
		cmd.Wait()
		if r, ok := cmd.Stdout.(io.WriteCloser); ok {
			r.Close()
		}
		if r, ok := cmd.Stderr.(io.WriteCloser); ok {
			r.Close()
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		log.Fatalf("failed to wait go command: %v", err)
	}

	exitCode := cmd.ProcessState.ExitCode()
	os.Exit(exitCode)
}
