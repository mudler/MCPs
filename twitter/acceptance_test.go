package main

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Twitter acceptance", func() {
	BeforeEach(func() {
		if os.Getenv("TWITTER_ACCEPTANCE") != "true" {
			Skip("Set TWITTER_ACCEPTANCE=true and Twitter credentials to run acceptance tests")
		}
		if !InitClientFromEnv() {
			Skip("Twitter acceptance tests require TWITTER_BEARER_TOKEN or OAuth 1.0a env (TWITTER_API_KEY, TWITTER_API_SECRET, TWITTER_ACCESS_TOKEN, TWITTER_ACCESS_SECRET)")
		}
	})

	Describe("Reading tools", func() {
		It("get_profile returns a known user", func() {
			ctx := context.Background()
			_, out, err := GetProfile(ctx, nil, GetProfileInput{Username: "TwitterDev"})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.User.ID).NotTo(BeEmpty())
			Expect(out.User.Username).To(Equal("TwitterDev"))
		})

		It("get_tweets returns tweets for a user", func() {
			ctx := context.Background()
			_, out, err := GetTweets(ctx, nil, GetTweetsInput{Username: "TwitterDev", MaxResults: 5})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Tweets).NotTo(BeNil())
			Expect(out.Count).To(BeNumerically("<=", 5))
		})

		It("search_tweets returns structure", func() {
			ctx := context.Background()
			_, out, err := SearchTweets(ctx, nil, SearchTweetsInput{Query: "twitter", MaxResults: 5})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Tweets).NotTo(BeNil())
			Expect(out.Count).To(BeNumerically("<=", 5))
		})

		It("get_user_relationships returns structure", func() {
			ctx := context.Background()
			_, out, err := GetUserRelationships(ctx, nil, GetUserRelationshipsInput{UserID: "783214", Type: "followers", MaxResults: 5})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Users).NotTo(BeNil())
		})
	})

	Describe("Timeline tools", func() {
		It("get_timeline with type user returns tweets", func() {
			ctx := context.Background()
			_, out, err := GetTimeline(ctx, nil, GetTimelineInput{TimelineType: "user", UserID: "783214", MaxResults: 5})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Tweets).NotTo(BeNil())
			Expect(out.Count).To(BeNumerically("<=", 5))
		})

		It("get_unanswered_mentions returns structure when user context present", func() {
			if !hasUserCtx {
				Skip("get_unanswered_mentions requires user context (OAuth 1.0a)")
			}
			ctx := context.Background()
			_, out, err := GetUnansweredMentions(ctx, nil, struct{}{})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Tweets).NotTo(BeNil())
			Expect(out.Count).To(BeNumerically("<=", maxTweets))
		})
	})

	Describe("Trends", func() {
		It("get_trends returns structure when OAuth 1.0a present", func() {
			if v1Client == nil {
				Skip("get_trends requires OAuth 1.0a (v1.1 API)")
			}
			ctx := context.Background()
			_, out, err := GetTrends(ctx, nil, GetTrendsInput{WOEID: 1})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Trends).NotTo(BeNil())
		})
	})

	Describe("Basic features (post, reply, user timeline, mentions)", func() {
		It("get user timeline returns tweets for authenticated user", func() {
			if !hasUserCtx || authUserID == "" {
				Skip("requires user context with resolved auth user ID")
			}
			ctx := context.Background()
			_, out, err := GetTimeline(ctx, nil, GetTimelineInput{TimelineType: "user", UserID: authUserID, MaxResults: 5})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Tweets).NotTo(BeNil())
			Expect(out.Count).To(BeNumerically("<=", 5))
		})

		It("get mentions returns structure", func() {
			if !hasUserCtx {
				Skip("get mentions requires user context (OAuth 1.0a)")
			}
			if authUserID == "" {
				Skip("auth user ID not resolved (rate limited); get_mentions needs it")
			}
			ctx := context.Background()
			_, out, err := GetUnansweredMentions(ctx, nil, struct{}{})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Tweets).NotTo(BeNil())
		})

		It("post_tweet creates a tweet", func() {
			if !hasUserCtx {
				Skip("post_tweet requires user context (OAuth 1.0a)")
			}
			ctx := context.Background()
			_, out, err := PostTweet(ctx, nil, PostTweetInput{Text: "MCP acceptance test tweet – please ignore"})
			Expect(err).NotTo(HaveOccurred())
			Expect(out.TweetID).NotTo(BeEmpty())
			Expect(out.Text).NotTo(BeEmpty())
			// Clean up: delete the tweet so we don't leave garbage
			_, delErr := client.DeleteTweet(ctx, out.TweetID)
			Expect(delErr).NotTo(HaveOccurred())
		})

		It("post_tweet with in_reply_to creates a reply", func() {
			if !hasUserCtx {
				Skip("post_tweet requires user context (OAuth 1.0a)")
			}
			ctx := context.Background()
			// Post original tweet
			_, orig, err := PostTweet(ctx, nil, PostTweetInput{Text: "MCP test thread – ignore"})
			Expect(err).NotTo(HaveOccurred())
			Expect(orig.TweetID).NotTo(BeEmpty())
			// Reply to it
			_, replyOut, err := PostTweet(ctx, nil, PostTweetInput{
				Text:             "MCP test reply – ignore",
				InReplyToTweetID: orig.TweetID,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(replyOut.TweetID).NotTo(BeEmpty())
			// Clean up: delete reply first, then original
			_, _ = client.DeleteTweet(ctx, replyOut.TweetID)
			_, delErr := client.DeleteTweet(ctx, orig.TweetID)
			Expect(delErr).NotTo(HaveOccurred())
		})
	})
})
