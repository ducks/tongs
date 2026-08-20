package repository

import (
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/ducks/tongs/internal/provider"
)

var scpRemotePattern = regexp.MustCompile(`^(?:[^@]+@)?([^:]+):(.+)$`)

type Options struct {
	Provider string
	Host     string
	Repo     string
	Remote   string
}

type Target struct {
	Repository provider.Repository
	Number     int
}

func Resolve(opts Options, selector string) (Target, error) {
	var repo provider.Repository
	var number int
	var err error

	if isPullRequestURL(selector) {
		repo, number, err = parsePullRequestURL(selector, opts.Provider)
		if err != nil {
			return Target{}, err
		}
	} else {
		repo, err = resolveRepository(opts)
		if err != nil {
			return Target{}, err
		}
		if selector != "" {
			number, err = strconv.Atoi(selector)
			if err != nil || number < 1 {
				return Target{}, fmt.Errorf("pull request selector must be a positive number or URL, got %q", selector)
			}
		}
	}

	applyOverrides(&repo, opts)
	if repo.Provider == "" {
		repo.Provider = providerForHost(repo.Host)
	}
	if repo.Provider == "" {
		return Target{}, fmt.Errorf("could not detect a provider for %q; use --provider", repo.Host)
	}

	return Target{Repository: repo, Number: number}, nil
}

func CurrentBranch() (string, error) {
	output, err := git("branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(output)
	if branch == "" {
		return "", fmt.Errorf("HEAD is detached; provide a pull request number or URL")
	}
	return branch, nil
}

func resolveRepository(opts Options) (provider.Repository, error) {
	if opts.Repo != "" {
		return parseRepositoryFlag(opts.Repo, opts.Host, opts.Provider)
	}

	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}
	remoteURL, err := git("config", "--get", "remote."+remote+".url")
	if err != nil {
		return provider.Repository{}, fmt.Errorf("could not read git remote %q: %w", remote, err)
	}
	return parseRemote(strings.TrimSpace(remoteURL))
}

func parseRepositoryFlag(value, host, providerName string) (provider.Repository, error) {
	if strings.Contains(value, "://") || strings.Contains(value, "@") {
		repo, err := parseRemote(value)
		if err != nil {
			return provider.Repository{}, err
		}
		applyOverrides(&repo, Options{Host: host, Provider: providerName})
		return repo, nil
	}

	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) == 3 {
		host = parts[0]
		parts = parts[1:]
	}
	if len(parts) != 2 {
		return provider.Repository{}, fmt.Errorf("repository must be OWNER/NAME, HOST/OWNER/NAME, or a git URL")
	}
	if host == "" {
		host = defaultHost(providerName)
	}
	return provider.Repository{
		Provider: providerName,
		Host:     host,
		Owner:    parts[0],
		Name:     strings.TrimSuffix(parts[1], ".git"),
	}, nil
}

func parseRemote(value string) (provider.Repository, error) {
	var host, repoPath string
	if match := scpRemotePattern.FindStringSubmatch(value); match != nil && !strings.Contains(value, "://") {
		host, repoPath = match[1], match[2]
	} else {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Hostname() == "" {
			return provider.Repository{}, fmt.Errorf("unsupported git remote URL %q", value)
		}
		host, repoPath = parsed.Hostname(), parsed.Path
	}

	parts := strings.Split(strings.Trim(strings.TrimSuffix(repoPath, ".git"), "/"), "/")
	if len(parts) < 2 {
		return provider.Repository{}, fmt.Errorf("could not determine owner and repository from %q", value)
	}
	return provider.Repository{
		Provider: providerForHost(host),
		Host:     host,
		Owner:    strings.Join(parts[:len(parts)-1], "/"),
		Name:     parts[len(parts)-1],
	}, nil
}

func parsePullRequestURL(value, providerOverride string) (provider.Repository, int, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return provider.Repository{}, 0, fmt.Errorf("invalid pull request URL %q", value)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	var owner, name, numberValue string
	providerName := providerOverride
	if providerName == "" {
		providerName = providerForHost(parsed.Hostname())
	}
	switch providerName {
	case "github":
		index := indexOf(parts, "pull")
		if index < 2 || index+1 >= len(parts) {
			return provider.Repository{}, 0, fmt.Errorf("invalid GitHub pull request URL %q", value)
		}
		owner, name, numberValue = strings.Join(parts[:index-1], "/"), parts[index-1], parts[index+1]
	case "gitlab":
		index := indexOf(parts, "merge_requests")
		if index < 3 || parts[index-1] != "-" || index+1 >= len(parts) {
			return provider.Repository{}, 0, fmt.Errorf("invalid GitLab merge request URL %q", value)
		}
		owner, name, numberValue = strings.Join(parts[:index-2], "/"), parts[index-2], parts[index+1]
	case "gitea":
		index := indexOf(parts, "pulls")
		if index < 2 || index+1 >= len(parts) {
			return provider.Repository{}, 0, fmt.Errorf("invalid Gitea pull request URL %q", value)
		}
		owner, name, numberValue = strings.Join(parts[:index-1], "/"), parts[index-1], parts[index+1]
	default:
		return provider.Repository{}, 0, fmt.Errorf("unknown provider for pull request URL %q; use --provider with a number", value)
	}

	number, err := strconv.Atoi(numberValue)
	if err != nil || number < 1 || owner == "" || name == "" {
		return provider.Repository{}, 0, fmt.Errorf("invalid pull request URL %q", value)
	}
	return provider.Repository{
		Provider: providerName,
		Host:     parsed.Hostname(),
		Owner:    owner,
		Name:     name,
	}, number, nil
}

func indexOf(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
}

func isPullRequestURL(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func applyOverrides(repo *provider.Repository, opts Options) {
	if opts.Provider != "" {
		repo.Provider = opts.Provider
	}
	if opts.Host != "" {
		repo.Host = opts.Host
	}
}

func providerForHost(host string) string {
	switch strings.ToLower(host) {
	case "github.com":
		return "github"
	case "gitlab.com":
		return "gitlab"
	case "codeberg.org":
		return "gitea"
	default:
		return ""
	}
}

func defaultHost(providerName string) string {
	switch providerName {
	case "gitlab":
		return "gitlab.com"
	case "gitea":
		return "codeberg.org"
	default:
		return "github.com"
	}
}

func git(args ...string) (string, error) {
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func TargetName(repo provider.Repository, number int) string {
	return path.Join(repo.Host, repo.Owner, repo.Name) + "#" + strconv.Itoa(number)
}
