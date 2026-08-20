package provider

import (
	"context"
	"fmt"
)

type Provider interface {
	Name() string
	Create(context.Context, Repository, CreateInput) (PullRequest, error)
	FindPullRequest(context.Context, Repository, string) (PullRequest, error)
	GetPullRequest(context.Context, Repository, int) (PullRequest, error)
	Inspect(context.Context, Repository, int) (Inspection, error)
	Edit(context.Context, Repository, int, EditInput) (PullRequest, error)
	Reviews(context.Context, Repository, int) ([]Review, error)
	Threads(context.Context, Repository, int) ([]ReviewThread, error)
	Reply(context.Context, Repository, int, string, string) (ReviewComment, error)
	Resolve(context.Context, Repository, int, string) (ReviewThread, error)
	Approve(context.Context, Repository, int, ApprovalInput) (Review, error)
	Checks(context.Context, Repository, string) (Checks, error)
	Merge(context.Context, Repository, int, MergeInput) (MergeResult, error)
}

type Factory func(host string) (Provider, error)

type Registry struct {
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: map[string]Factory{}}
}

func (r *Registry) Register(name string, factory Factory) {
	r.factories[name] = factory
}

func (r *Registry) Create(name, host string) (Provider, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, &Error{Code: "provider_not_implemented", Message: fmt.Sprintf("provider %q is not implemented", name)}
	}
	return factory(host)
}
