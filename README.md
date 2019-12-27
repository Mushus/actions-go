# actions-go

Translate go command errors into Github Actions reports

# How dose it work?

`actions-go` is used like `go` command.

`build` or `run` subcommand

```bash
$ actions-go run main.go
# command-line-arguments
::error file=main.go,file=349,col=3:: unexpected a at end of statement
./main.go:349:3: syntax error: unexpected a at end of statement
```

`test` subcommand

```bash
$ actions-go test ./test/...
--- FAIL: TestHoge (0.00s)
    main_test.go:6: test failure
FAIL
::error file=test/main_test.go,line=6:: test failure
FAIL    github.com/Mushus/actions-go/test       0.008s
FAIL
```
