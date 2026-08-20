package github

import (
	"context"
	"time"

	"github.com/ducks/tongs/internal/provider"
)

const reviewThreadsQuery = `
query ReviewThreads($owner: String!, $name: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviewThreads(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          isOutdated
          path
          line
          comments(first: 100) {
            nodes {
              id
              databaseId
              body
              path
              line
              url
              createdAt
              author { login }
            }
          }
        }
      }
    }
  }
}`

type graphComment struct {
	ID         string    `json:"id"`
	DatabaseID int64     `json:"databaseId"`
	Body       string    `json:"body"`
	Path       string    `json:"path"`
	Line       *int      `json:"line"`
	URL        string    `json:"url"`
	CreatedAt  time.Time `json:"createdAt"`
	Author     struct {
		Login string `json:"login"`
	} `json:"author"`
}

type graphThread struct {
	ID         string `json:"id"`
	IsResolved bool   `json:"isResolved"`
	IsOutdated bool   `json:"isOutdated"`
	Path       string `json:"path"`
	Line       *int   `json:"line"`
	Comments   struct {
		Nodes []graphComment `json:"nodes"`
	} `json:"comments"`
}

func (c *Client) Threads(ctx context.Context, repo provider.Repository, number int) ([]provider.ReviewThread, error) {
	variables := map[string]interface{}{
		"owner":  repo.Owner,
		"name":   repo.Name,
		"number": number,
		"after":  nil,
	}
	threads := []provider.ReviewThread{}

	for {
		var response struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						Nodes    []graphThread `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}
		if err := c.graphql(ctx, reviewThreadsQuery, variables, &response); err != nil {
			return nil, err
		}

		connection := response.Repository.PullRequest.ReviewThreads
		for _, thread := range connection.Nodes {
			threads = append(threads, mapThread(thread))
		}
		if !connection.PageInfo.HasNextPage {
			break
		}
		variables["after"] = connection.PageInfo.EndCursor
	}
	return threads, nil
}

func (c *Client) Reply(ctx context.Context, _ provider.Repository, _ int, threadID, body string) (provider.ReviewComment, error) {
	const mutation = `
mutation ReplyToReviewThread($threadId: ID!, $body: String!) {
  addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $threadId, body: $body}) {
    comment {
      id
      databaseId
      body
      path
      line
      url
      createdAt
      author { login }
    }
  }
}`
	var response struct {
		Reply struct {
			Comment graphComment `json:"comment"`
		} `json:"addPullRequestReviewThreadReply"`
	}
	if err := c.graphql(ctx, mutation, map[string]interface{}{"threadId": threadID, "body": body}, &response); err != nil {
		return provider.ReviewComment{}, err
	}
	return mapComment(response.Reply.Comment), nil
}

func (c *Client) Resolve(ctx context.Context, _ provider.Repository, _ int, threadID string) (provider.ReviewThread, error) {
	const mutation = `
mutation ResolveReviewThread($threadId: ID!) {
  resolveReviewThread(input: {threadId: $threadId}) {
    thread { id isResolved path line }
  }
}`
	var response struct {
		Resolve struct {
			Thread graphThread `json:"thread"`
		} `json:"resolveReviewThread"`
	}
	if err := c.graphql(ctx, mutation, map[string]interface{}{"threadId": threadID}, &response); err != nil {
		return provider.ReviewThread{}, err
	}
	return mapThread(response.Resolve.Thread), nil
}

func mapThread(thread graphThread) provider.ReviewThread {
	comments := make([]provider.ReviewComment, 0, len(thread.Comments.Nodes))
	for _, comment := range thread.Comments.Nodes {
		comments = append(comments, mapComment(comment))
	}
	return provider.ReviewThread{
		ID:       thread.ID,
		Resolved: thread.IsResolved,
		Outdated: thread.IsOutdated,
		Path:     thread.Path,
		Line:     thread.Line,
		Comments: comments,
	}
}

func mapComment(comment graphComment) provider.ReviewComment {
	return provider.ReviewComment{
		ID:         comment.ID,
		DatabaseID: comment.DatabaseID,
		Author:     provider.User{Login: comment.Author.Login},
		Body:       comment.Body,
		Path:       comment.Path,
		Line:       comment.Line,
		URL:        comment.URL,
		CreatedAt:  comment.CreatedAt,
	}
}
