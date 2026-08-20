package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ducks/tongs/internal/provider"
	"github.com/ducks/tongs/internal/provider/github"
	"github.com/ducks/tongs/internal/repository"
)

const version = "0.1.0"

type app struct {
	ctx      context.Context
	stdin    io.Reader
	out      output
	registry *provider.Registry
	opts     globalOptions
}

type globalOptions struct {
	provider string
	host     string
	repo     string
	remote   string
	pretty   bool
	dryRun   bool
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	global := flag.NewFlagSet("tongs", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	opts := globalOptions{}
	global.StringVar(&opts.provider, "provider", "", "provider adapter")
	global.StringVar(&opts.host, "host", "", "provider hostname")
	global.StringVar(&opts.repo, "repo", "", "OWNER/NAME, HOST/OWNER/NAME, or git URL")
	global.StringVar(&opts.remote, "remote", "origin", "git remote used for repository detection")
	global.BoolVar(&opts.pretty, "pretty", false, "pretty-print JSON output")
	global.BoolVar(&opts.dryRun, "dry-run", false, "describe mutations without applying them")
	showVersion := global.Bool("version", false, "print version")
	if err := global.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_ = newOutput(stdout, stderr, opts.pretty).success(map[string]interface{}{"usage": usageText(), "providers": []string{"github"}}, false)
			return 0
		}
		newOutput(stdout, stderr, opts.pretty).usage(err.Error())
		return 2
	}

	out := newOutput(stdout, stderr, opts.pretty)
	if *showVersion {
		_ = out.success(map[string]string{"version": version}, false)
		return 0
	}
	remaining := global.Args()
	if len(remaining) == 0 {
		out.usage("missing command; expected inspect, edit, reviews, reply, resolve, checks, or merge")
		return 2
	}

	registry := provider.NewRegistry()
	registry.Register("github", func(host string) (provider.Provider, error) { return github.New(host) })
	application := app{ctx: ctx, stdin: stdin, out: out, registry: registry, opts: opts}
	if err := application.run(remaining[0], remaining[1:]); err != nil {
		var usageErr *usageError
		if errors.As(err, &usageErr) {
			out.usage(usageErr.Error())
			return 2
		}
		out.failure(err)
		return exitCode(err)
	}
	return 0
}

func newOutput(stdout, stderr io.Writer, pretty bool) output {
	return output{stdout: stdout, stderr: stderr, pretty: pretty}
}

func (a *app) run(command string, args []string) error {
	switch command {
	case "inspect":
		return a.inspect(args)
	case "edit":
		return a.edit(args)
	case "reviews":
		return a.reviews(args)
	case "reply":
		return a.reply(args)
	case "resolve":
		return a.resolve(args)
	case "checks":
		return a.checks(args)
	case "merge":
		return a.merge(args)
	case "help", "--help", "-h":
		return a.out.success(map[string]interface{}{"usage": usageText(), "providers": []string{"github"}}, false)
	default:
		return usagef("unknown command %q", command)
	}
}

func (a *app) resolveTarget(selector string) (repository.Target, provider.Provider, error) {
	target, err := repository.Resolve(repository.Options{
		Provider: a.opts.provider,
		Host:     a.opts.host,
		Repo:     a.opts.repo,
		Remote:   a.opts.remote,
	}, selector)
	if err != nil {
		return repository.Target{}, nil, err
	}
	adapter, err := a.registry.Create(target.Repository.Provider, target.Repository.Host)
	if err != nil {
		return repository.Target{}, nil, err
	}
	if target.Number == 0 {
		branch, err := repository.CurrentBranch()
		if err != nil {
			return repository.Target{}, nil, err
		}
		pull, err := adapter.FindPullRequest(a.ctx, target.Repository, branch)
		if err != nil {
			return repository.Target{}, nil, err
		}
		target.Number = pull.Number
	}
	return target, adapter, nil
}

func selector(args []string) (string, error) {
	if len(args) > 1 {
		return "", usagef("expected at most one pull request number or URL")
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return "", nil
}

type usageError struct{ message string }

func (e *usageError) Error() string { return e.message }

func usagef(format string, values ...interface{}) error {
	return &usageError{message: fmt.Sprintf(format, values...)}
}

func parseFlags(name string, args []string, configure func(*flag.FlagSet)) (*flag.FlagSet, error) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	configure(set)
	if err := set.Parse(args); err != nil {
		return nil, usagef("%s: %v", name, err)
	}
	return set, nil
}

func wasSet(set *flag.FlagSet, name string) bool {
	found := false
	set.Visit(func(item *flag.Flag) {
		if item.Name == name {
			found = true
		}
	})
	return found
}

func readValue(inline, filename string, stdin io.Reader) (string, error) {
	if inline != "" && filename != "" {
		return "", usagef("use either an inline value or a file, not both")
	}
	if filename == "" {
		return inline, nil
	}
	var data []byte
	var err error
	if filename == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(filename)
	}
	if err != nil {
		return "", fmt.Errorf("read %q: %w", filename, err)
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

func exitCode(err error) int {
	var structured *provider.Error
	if errors.As(err, &structured) && structured.Code == "checks_failed" {
		return 3
	}
	return 1
}

func usageText() string {
	return `tongs [global flags] COMMAND [command flags] [PR]

Global flags must precede COMMAND:
  --provider NAME   Provider adapter (auto-detected for known hosts)
  --host HOST       Provider hostname
  --repo REPO       OWNER/NAME, HOST/OWNER/NAME, or git URL
  --remote NAME     Git remote for detection (default: origin)
  --pretty          Pretty-print JSON
  --dry-run         Preview mutations

Commands:
  inspect [PR]      PR, reviews, threads, and checks
  edit [PR]         Edit title, body, or state
  reviews [PR]      Reviews and review threads
  reply [PR]        Reply to a review thread
  resolve [PR]      Resolve a review thread
  checks [PR]       Check runs and commit statuses
  merge [PR]        Merge with an explicit --yes

PR may be a number or URL. When omitted, the open PR for the current branch is used.`
}
