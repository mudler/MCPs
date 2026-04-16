package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type stubCall struct {
	method string
	path   string
	accept string
	auth   string
	query  string
}

func newStubServer(handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *[]stubCall) {
	calls := &[]stubCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, stubCall{
			method: r.Method,
			path:   r.URL.Path,
			accept: r.Header.Get("Accept"),
			auth:   r.Header.Get("Authorization"),
			query:  r.URL.RawQuery,
		})
		handler(w, r)
	}))
	return srv, calls
}

func installClient(baseURL, token string) {
	client = &GitHubClient{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
}

var _ = Describe("parseGitHubURL", func() {
	It("parses a valid issue URL", func() {
		owner, repo, number, kind, err := parseGitHubURL("https://github.com/acme/widgets/issues/42")
		Expect(err).NotTo(HaveOccurred())
		Expect(owner).To(Equal("acme"))
		Expect(repo).To(Equal("widgets"))
		Expect(number).To(Equal(42))
		Expect(kind).To(Equal(kindIssue))
	})

	It("parses a valid pull request URL with trailing fragment", func() {
		owner, repo, number, kind, err := parseGitHubURL("https://github.com/foo/bar/pull/7#issuecomment-123")
		Expect(err).NotTo(HaveOccurred())
		Expect(owner).To(Equal("foo"))
		Expect(repo).To(Equal("bar"))
		Expect(number).To(Equal(7))
		Expect(kind).To(Equal(kindPullRequest))
	})

	It("rejects a malformed URL", func() {
		_, _, _, _, err := parseGitHubURL("https://example.com/not-github")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("GetIssue", func() {
	var (
		srv   *httptest.Server
		calls *[]stubCall
	)

	BeforeEach(func() {
		maxCommentLen = defaultMaxCommentLength

		issue := ghIssue{
			Number:    42,
			Title:     "Something is broken",
			State:     "open",
			Body:      "Steps to reproduce...",
			HTMLURL:   "https://github.com/acme/widgets/issues/42",
			User:      ghUser{Login: "alice"},
			Labels:    []ghLabel{{Name: "bug"}, {Name: "triage"}},
			CreatedAt: "2026-04-01T00:00:00Z",
			Comments:  2,
		}
		comments := []ghComment{
			{User: ghUser{Login: "bob"}, Body: "I can repro", CreatedAt: "2026-04-02T00:00:00Z"},
			{User: ghUser{Login: "alice"}, Body: "Thanks", CreatedAt: "2026-04-03T00:00:00Z"},
		}

		srv, calls = newStubServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/issues/42":
				_ = json.NewEncoder(w).Encode(issue)
			case "/repos/acme/widgets/issues/42/comments":
				_ = json.NewEncoder(w).Encode(comments)
			default:
				http.NotFound(w, r)
			}
		})
		installClient(srv.URL, "")
	})

	AfterEach(func() { srv.Close() })

	It("returns the issue with its comments", func() {
		_, out, err := GetIssue(context.Background(), nil, GetIssueInput{
			Owner: "acme", Repo: "widgets", Number: 42,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Issue.Number).To(Equal(42))
		Expect(out.Issue.Title).To(Equal("Something is broken"))
		Expect(out.Issue.Author).To(Equal("alice"))
		Expect(out.Issue.Labels).To(ConsistOf("bug", "triage"))
		Expect(out.Issue.Body).To(ContainSubstring("reproduce"))
		Expect(out.Issue.CommentsTotal).To(Equal(2))
		Expect(out.Issue.Comments).To(HaveLen(2))
		Expect(out.Issue.Comments[0].Author).To(Equal("bob"))

		Expect(*calls).To(HaveLen(2))
		for _, c := range *calls {
			Expect(c.auth).To(BeEmpty(), "no auth header should be sent when no token is configured")
			Expect(c.accept).To(Equal(defaultAccept))
		}
	})

	It("also accepts a URL input", func() {
		_, out, err := GetIssue(context.Background(), nil, GetIssueInput{
			URL: "https://github.com/acme/widgets/issues/42",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Issue.Number).To(Equal(42))
	})

	It("rejects a PR URL passed to get_issue", func() {
		_, _, err := GetIssue(context.Background(), nil, GetIssueInput{
			URL: "https://github.com/acme/widgets/pull/42",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("expects a issue URL"))
	})

	It("sends the auth header when a token is configured", func() {
		installClient(srv.URL, "test-token-123")
		_, _, err := GetIssue(context.Background(), nil, GetIssueInput{
			Owner: "acme", Repo: "widgets", Number: 42,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(*calls).NotTo(BeEmpty())
		Expect((*calls)[0].auth).To(Equal("Bearer test-token-123"))
	})

	It("truncates long comment bodies", func() {
		maxCommentLen = 10
		longBody := strings.Repeat("x", 500)
		srv.Close()
		srv, calls = newStubServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/issues/42":
				_ = json.NewEncoder(w).Encode(ghIssue{
					Number: 42, Title: "t", State: "open", Comments: 1,
					User: ghUser{Login: "alice"},
				})
			case "/repos/acme/widgets/issues/42/comments":
				_ = json.NewEncoder(w).Encode([]ghComment{
					{User: ghUser{Login: "bob"}, Body: longBody},
				})
			default:
				http.NotFound(w, r)
			}
		})
		installClient(srv.URL, "")

		_, out, err := GetIssue(context.Background(), nil, GetIssueInput{
			Owner: "acme", Repo: "widgets", Number: 42,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Issue.Comments).To(HaveLen(1))
		Expect(out.Issue.Comments[0].Truncated).To(BeTrue())
		Expect(len(out.Issue.Comments[0].Body)).To(BeNumerically("<", len(longBody)))
	})
})

var _ = Describe("GetPullRequest", func() {
	var (
		srv   *httptest.Server
		calls *[]stubCall
	)

	const canonicalDiff = "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@\n-old\n+new\n"

	prHandler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets/pulls/7":
			if strings.Contains(r.Header.Get("Accept"), "diff") {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = fmt.Fprint(w, canonicalDiff)
				return
			}
			_ = json.NewEncoder(w).Encode(ghPullRequest{
				Number:  7,
				Title:   "Add feature",
				State:   "open",
				Body:    "Implements X",
				HTMLURL: "https://github.com/acme/widgets/pull/7",
				User:    ghUser{Login: "carol"},
				Labels:  []ghLabel{{Name: "enhancement"}},
				Merged:  false,
				Draft:   true,
				Base:    ghRef{Ref: "main"},
				Head:    ghRef{Ref: "feat/x"},
			})
		case "/repos/acme/widgets/issues/7/comments":
			_ = json.NewEncoder(w).Encode([]ghComment{
				{User: ghUser{Login: "dan"}, Body: "LGTM pending tests"},
			})
		default:
			http.NotFound(w, r)
		}
	}

	BeforeEach(func() {
		maxCommentLen = defaultMaxCommentLength
		srv, calls = newStubServer(prHandler)
		installClient(srv.URL, "")
	})

	AfterEach(func() { srv.Close() })

	It("returns the PR with comments and without diff by default", func() {
		_, out, err := GetPullRequest(context.Background(), nil, GetPullRequestInput{
			Owner: "acme", Repo: "widgets", Number: 7,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.PullRequest.Number).To(Equal(7))
		Expect(out.PullRequest.Draft).To(BeTrue())
		Expect(out.PullRequest.BaseRef).To(Equal("main"))
		Expect(out.PullRequest.HeadRef).To(Equal("feat/x"))
		Expect(out.PullRequest.Labels).To(ConsistOf("enhancement"))
		Expect(out.PullRequest.CommentsTotal).To(Equal(1))
		Expect(out.PullRequest.Diff).To(BeEmpty())

		for _, c := range *calls {
			Expect(c.accept).NotTo(ContainSubstring("diff"), "diff endpoint should not be hit when include_diff is false")
		}
	})

	It("returns the diff when include_diff is set", func() {
		_, out, err := GetPullRequest(context.Background(), nil, GetPullRequestInput{
			Owner: "acme", Repo: "widgets", Number: 7, IncludeDiff: true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.PullRequest.Diff).To(Equal(canonicalDiff))

		sawDiffRequest := false
		for _, c := range *calls {
			if c.path == "/repos/acme/widgets/pulls/7" && strings.Contains(c.accept, "diff") {
				sawDiffRequest = true
			}
		}
		Expect(sawDiffRequest).To(BeTrue())
	})

	It("errors when neither url nor owner/repo/number is provided", func() {
		_, _, err := GetPullRequest(context.Background(), nil, GetPullRequestInput{})
		Expect(err).To(HaveOccurred())
	})
})
