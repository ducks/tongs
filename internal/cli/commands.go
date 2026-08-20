package cli

import (
	"flag"
	"strings"

	"github.com/ducks/tongs/internal/provider"
	"github.com/ducks/tongs/internal/repository"
)

func (a *app) inspect(args []string) error {
	selector, err := selector(args)
	if err != nil {
		return err
	}
	target, adapter, err := a.resolveTarget(selector)
	if err != nil {
		return err
	}
	result, err := adapter.Inspect(a.ctx, target.Repository, target.Number)
	if err != nil {
		return err
	}
	return a.out.success(result, false)
}

func (a *app) edit(args []string) error {
	var title, body, bodyFile, state string
	set, err := parseFlags("edit", args, func(set *flag.FlagSet) {
		set.StringVar(&title, "title", "", "new pull request title")
		set.StringVar(&body, "body", "", "new pull request body")
		set.StringVar(&bodyFile, "body-file", "", "read body from a file, or - for stdin")
		set.StringVar(&state, "state", "", "open or closed")
	})
	if err != nil {
		return err
	}
	selector, err := selector(set.Args())
	if err != nil {
		return err
	}
	if !wasSet(set, "title") && !wasSet(set, "body") && !wasSet(set, "body-file") && !wasSet(set, "state") {
		return usagef("edit requires --title, --body, --body-file, or --state")
	}
	if state != "" && state != "open" && state != "closed" {
		return usagef("edit --state must be open or closed")
	}

	input := provider.EditInput{}
	if wasSet(set, "title") {
		input.Title = &title
	}
	if wasSet(set, "body") || wasSet(set, "body-file") {
		value, err := readValue(body, bodyFile, a.stdin)
		if err != nil {
			return err
		}
		input.Body = &value
	}
	if wasSet(set, "state") {
		input.State = &state
	}

	target, adapter, err := a.resolveTarget(selector)
	if err != nil {
		return err
	}
	if a.opts.dryRun {
		return a.preview(target, "edit", input)
	}
	result, err := adapter.Edit(a.ctx, target.Repository, target.Number, input)
	if err != nil {
		return err
	}
	return a.out.success(result, false)
}

func (a *app) reviews(args []string) error {
	selector, err := selector(args)
	if err != nil {
		return err
	}
	target, adapter, err := a.resolveTarget(selector)
	if err != nil {
		return err
	}
	reviews, err := adapter.Reviews(a.ctx, target.Repository, target.Number)
	if err != nil {
		return err
	}
	threads, err := adapter.Threads(a.ctx, target.Repository, target.Number)
	if err != nil {
		return err
	}
	return a.out.success(map[string]interface{}{
		"repository": target.Repository,
		"number":     target.Number,
		"reviews":    reviews,
		"threads":    threads,
	}, false)
}

func (a *app) reply(args []string) error {
	var threadID, body, bodyFile string
	set, err := parseFlags("reply", args, func(set *flag.FlagSet) {
		set.StringVar(&threadID, "thread", "", "review thread node ID")
		set.StringVar(&body, "body", "", "reply body")
		set.StringVar(&bodyFile, "body-file", "", "read reply from a file, or - for stdin")
	})
	if err != nil {
		return err
	}
	selector, err := selector(set.Args())
	if err != nil {
		return err
	}
	if threadID == "" {
		return usagef("reply requires --thread")
	}
	if !wasSet(set, "body") && !wasSet(set, "body-file") {
		return usagef("reply requires --body or --body-file")
	}
	value, err := readValue(body, bodyFile, a.stdin)
	if err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		return usagef("reply body cannot be empty")
	}

	target, adapter, err := a.resolveTarget(selector)
	if err != nil {
		return err
	}
	input := map[string]string{"thread_id": threadID, "body": value}
	if a.opts.dryRun {
		return a.preview(target, "reply", input)
	}
	result, err := adapter.Reply(a.ctx, target.Repository, target.Number, threadID, value)
	if err != nil {
		return err
	}
	return a.out.success(result, false)
}

func (a *app) resolve(args []string) error {
	var threadID string
	set, err := parseFlags("resolve", args, func(set *flag.FlagSet) {
		set.StringVar(&threadID, "thread", "", "review thread node ID")
	})
	if err != nil {
		return err
	}
	selector, err := selector(set.Args())
	if err != nil {
		return err
	}
	if threadID == "" {
		return usagef("resolve requires --thread")
	}

	target, adapter, err := a.resolveTarget(selector)
	if err != nil {
		return err
	}
	if a.opts.dryRun {
		return a.preview(target, "resolve", map[string]string{"thread_id": threadID})
	}
	result, err := adapter.Resolve(a.ctx, target.Repository, target.Number, threadID)
	if err != nil {
		return err
	}
	return a.out.success(result, false)
}

func (a *app) checks(args []string) error {
	selector, err := selector(args)
	if err != nil {
		return err
	}
	target, adapter, err := a.resolveTarget(selector)
	if err != nil {
		return err
	}
	pull, err := adapter.GetPullRequest(a.ctx, target.Repository, target.Number)
	if err != nil {
		return err
	}
	result, err := adapter.Checks(a.ctx, target.Repository, pull.HeadSHA)
	if err != nil {
		return err
	}
	data := map[string]interface{}{
		"repository": target.Repository,
		"number":     target.Number,
		"head_sha":   pull.HeadSHA,
		"result":     result,
	}
	if result.Overall == "failure" {
		return &provider.Error{Code: "checks_failed", Message: "one or more checks failed", Details: data}
	}
	return a.out.success(data, false)
}

func (a *app) merge(args []string) error {
	var method, title, message, messageFile, expectedSHA string
	var yes bool
	set, err := parseFlags("merge", args, func(set *flag.FlagSet) {
		set.StringVar(&method, "method", "squash", "merge, squash, or rebase")
		set.StringVar(&title, "title", "", "merge commit title")
		set.StringVar(&message, "message", "", "merge commit message")
		set.StringVar(&messageFile, "message-file", "", "read merge message from a file, or - for stdin")
		set.StringVar(&expectedSHA, "sha", "", "require this head SHA")
		set.BoolVar(&yes, "yes", false, "confirm the merge")
	})
	if err != nil {
		return err
	}
	selector, err := selector(set.Args())
	if err != nil {
		return err
	}
	if method != "merge" && method != "squash" && method != "rebase" {
		return usagef("merge --method must be merge, squash, or rebase")
	}
	if !yes && !a.opts.dryRun {
		return usagef("merge requires --yes (or use --dry-run)")
	}
	messageValue, err := readValue(message, messageFile, a.stdin)
	if err != nil {
		return err
	}

	target, adapter, err := a.resolveTarget(selector)
	if err != nil {
		return err
	}
	if expectedSHA == "" {
		pull, err := adapter.GetPullRequest(a.ctx, target.Repository, target.Number)
		if err != nil {
			return err
		}
		expectedSHA = pull.HeadSHA
	}
	input := provider.MergeInput{Method: method, Title: title, Message: messageValue, ExpectedSHA: expectedSHA}
	if a.opts.dryRun {
		return a.preview(target, "merge", input)
	}
	result, err := adapter.Merge(a.ctx, target.Repository, target.Number, input)
	if err != nil {
		return err
	}
	return a.out.success(result, false)
}

func (a *app) preview(target repository.Target, action string, input interface{}) error {
	return a.out.success(provider.MutationPreview{
		Provider: target.Repository.Provider,
		Action:   action,
		Target:   repository.TargetName(target.Repository, target.Number),
		Input:    input,
	}, true)
}
