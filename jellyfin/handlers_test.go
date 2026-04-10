package main

import (
	"context"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These are E2E tests that run against a real Jellyfin instance.
// Required env vars: JELLYFIN_URL, JELLYFIN_API_KEY
// Optional: JELLYFIN_USER_ID (needed for user-scoped tests)

var _ = Describe("Jellyfin Handlers (E2E)", func() {
	BeforeEach(func() {
		jellyfinURL := os.Getenv("JELLYFIN_URL")
		apiKey := os.Getenv("JELLYFIN_API_KEY")
		if jellyfinURL == "" || apiKey == "" {
			Skip("JELLYFIN_URL and JELLYFIN_API_KEY must be set for E2E tests")
		}

		userID := os.Getenv("JELLYFIN_USER_ID")

		client = &JellyfinClient{
			BaseURL: jellyfinURL,
			APIKey:  apiKey,
			UserID:  userID,
			HTTPClient: &http.Client{
				Timeout: 30 * time.Second,
			},
		}

		// Resolve JELLYFIN_USERNAME if JELLYFIN_USER_ID is not set
		if client.UserID == "" {
			if username := os.Getenv("JELLYFIN_USERNAME"); username != "" {
				resolvedID, err := ResolveUsername(context.Background(), client, username)
				Expect(err).NotTo(HaveOccurred())
				client.UserID = resolvedID
			}
		}
	})

	Context("list_libraries", func() {
		It("should return at least one library", func() {
			_, output, err := ListLibraries(context.Background(), nil, ListLibrariesInput{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Count).To(BeNumerically(">", 0))
			Expect(output.Libraries).NotTo(BeEmpty())
			Expect(output.Libraries[0].ID).NotTo(BeEmpty())
			Expect(output.Libraries[0].Name).NotTo(BeEmpty())
		})
	})

	Context("search", func() {
		It("should return results for a broad query", func() {
			_, output, err := Search(context.Background(), nil, SearchInput{
				Query: "a",
				Limit: 5,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Count).To(BeNumerically(">", 0))
			Expect(output.Items[0].Name).NotTo(BeEmpty())
		})

		It("should respect the limit parameter", func() {
			_, output, err := Search(context.Background(), nil, SearchInput{
				Query: "a",
				Limit: 2,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Count).To(BeNumerically("<=", 2))
		})

		It("should return empty results for nonsense query", func() {
			_, output, err := Search(context.Background(), nil, SearchInput{
				Query: "zzzxxx999nonexistent",
				Limit: 5,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Count).To(Equal(0))
		})
	})

	Context("browse_library", func() {
		It("should browse all items with default params", func() {
			_, output, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
				ItemTypes: "Movie",
				Limit:     5,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.TotalCount).To(BeNumerically(">", 0))
			Expect(output.Items).NotTo(BeEmpty())
			Expect(output.Items[0].Type).To(Equal("Movie"))
		})

		It("should support pagination", func() {
			_, page1, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
				ItemTypes: "Movie",
				Limit:     2,
			})
			Expect(err).NotTo(HaveOccurred())

			_, page2, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
				ItemTypes:  "Movie",
				Limit:      2,
				StartIndex: 2,
			})
			Expect(err).NotTo(HaveOccurred())

			// Pages should have different items (assuming > 2 movies)
			if len(page1.Items) > 0 && len(page2.Items) > 0 {
				Expect(page1.Items[0].ID).NotTo(Equal(page2.Items[0].ID))
			}
		})

		It("should filter by parent library ID", func() {
			// First get libraries
			_, libs, err := ListLibraries(context.Background(), nil, ListLibrariesInput{})
			Expect(err).NotTo(HaveOccurred())
			Expect(libs.Libraries).NotTo(BeEmpty())

			// Browse the first library
			_, output, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
				ParentID: libs.Libraries[0].ID,
				Limit:    3,
			})
			Expect(err).NotTo(HaveOccurred())
			// Should not error (may have 0 items if library is empty)
			Expect(output.TotalCount).To(BeNumerically(">=", 0))
		})

		It("should sort by community rating descending", func() {
			_, output, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
				ItemTypes: "Movie",
				SortBy:    "CommunityRating",
				SortOrder: "Descending",
				Limit:     5,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Items).NotTo(BeEmpty())
		})
	})

	Context("get_item", func() {
		It("should return full details for a valid item", func() {
			// Browse for a movie to get a real item ID (search can return non-item types like Studio)
			_, browseOut, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
				ItemTypes: "Movie",
				Limit:     1,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(browseOut.Items).NotTo(BeEmpty())

			itemID := browseOut.Items[0].ID
			_, output, err := GetItem(context.Background(), nil, GetItemInput{ItemID: itemID})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Item.ID).To(Equal(itemID))
			Expect(output.Item.Name).NotTo(BeEmpty())
			Expect(output.Item.Type).To(Equal("Movie"))
		})

		It("should return an error for a nonexistent item", func() {
			_, _, err := GetItem(context.Background(), nil, GetItemInput{ItemID: "nonexistent-id-12345"})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("get_similar", func() {
		It("should return similar items for a movie", func() {
			// Find a movie first
			_, searchOut, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
				ItemTypes: "Movie",
				Limit:     1,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(searchOut.Items).NotTo(BeEmpty())

			_, output, err := GetSimilar(context.Background(), nil, GetSimilarInput{
				ItemID: searchOut.Items[0].ID,
				Limit:  5,
			})
			Expect(err).NotTo(HaveOccurred())
			// May have 0 similar items, that's OK
			Expect(output.Count).To(BeNumerically(">=", 0))
		})
	})

	Context("TV Shows", func() {
		var seriesID string

		BeforeEach(func() {
			// Find a series
			_, browseOut, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
				ItemTypes: "Series",
				Limit:     1,
			})
			Expect(err).NotTo(HaveOccurred())
			if len(browseOut.Items) == 0 {
				Skip("No TV series found on this server")
			}
			seriesID = browseOut.Items[0].ID
		})

		It("should list seasons for a series", func() {
			_, output, err := GetSeasons(context.Background(), nil, GetSeasonsInput{SeriesID: seriesID})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Count).To(BeNumerically(">", 0))
			Expect(output.Seasons[0].ID).NotTo(BeEmpty())
			Expect(output.Seasons[0].Name).NotTo(BeEmpty())
		})

		It("should list episodes for a series", func() {
			_, output, err := GetEpisodes(context.Background(), nil, GetEpisodesInput{SeriesID: seriesID})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Count).To(BeNumerically(">", 0))
			Expect(output.Episodes[0].Name).NotTo(BeEmpty())
		})

		It("should list episodes filtered by season", func() {
			_, seasons, err := GetSeasons(context.Background(), nil, GetSeasonsInput{SeriesID: seriesID})
			Expect(err).NotTo(HaveOccurred())
			Expect(seasons.Seasons).NotTo(BeEmpty())

			_, episodes, err := GetEpisodes(context.Background(), nil, GetEpisodesInput{
				SeriesID: seriesID,
				SeasonID: seasons.Seasons[0].ID,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(episodes.Count).To(BeNumerically(">", 0))
		})
	})

	Context("get_sessions", func() {
		It("should return sessions without error", func() {
			_, output, err := GetSessions(context.Background(), nil, GetSessionsInput{})
			Expect(err).NotTo(HaveOccurred())
			// There may or may not be active sessions
			Expect(output.Count).To(BeNumerically(">=", 0))
		})
	})

	Context("playback_control", func() {
		It("should reject invalid commands", func() {
			_, output, err := PlaybackControl(context.Background(), nil, PlaybackControlInput{
				SessionID: "fake-session",
				Command:   "InvalidCommand",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Success).To(BeFalse())
			Expect(output.Message).To(ContainSubstring("Invalid command"))
		})
	})

	Context("User-scoped operations", func() {
		BeforeEach(func() {
			if client.UserID == "" {
				Skip("JELLYFIN_USER_ID must be set for user-scoped tests")
			}
		})

		Context("get_latest", func() {
			It("should return recently added items", func() {
				_, output, err := GetLatest(context.Background(), nil, GetLatestInput{Limit: 5})
				Expect(err).NotTo(HaveOccurred())
				Expect(output.Count).To(BeNumerically(">", 0))
				Expect(output.Items[0].Name).NotTo(BeEmpty())
			})

			It("should filter by library", func() {
				_, libs, err := ListLibraries(context.Background(), nil, ListLibrariesInput{})
				Expect(err).NotTo(HaveOccurred())

				_, output, err := GetLatest(context.Background(), nil, GetLatestInput{
					ParentID: libs.Libraries[0].ID,
					Limit:    3,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(output.Count).To(BeNumerically(">=", 0))
			})
		})

		Context("get_next_up", func() {
			It("should return next up episodes without error", func() {
				_, output, err := GetNextUp(context.Background(), nil, GetNextUpInput{Limit: 5})
				Expect(err).NotTo(HaveOccurred())
				Expect(output.Count).To(BeNumerically(">=", 0))
			})
		})

		Context("set_favorite", func() {
			It("should toggle favorite on and off", func() {
				// Find an item
				_, browseOut, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
					ItemTypes: "Movie",
					Limit:     1,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(browseOut.Items).NotTo(BeEmpty())
				itemID := browseOut.Items[0].ID

				// Set favorite
				_, favOut, err := SetFavorite(context.Background(), nil, SetFavoriteInput{
					ItemID:   itemID,
					Favorite: true,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(favOut.Success).To(BeTrue())

				// Unset favorite
				_, unfavOut, err := SetFavorite(context.Background(), nil, SetFavoriteInput{
					ItemID:   itemID,
					Favorite: false,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(unfavOut.Success).To(BeTrue())
			})
		})

		Context("set_played", func() {
			It("should toggle played on and off", func() {
				// Find an item
				_, browseOut, err := BrowseLibrary(context.Background(), nil, BrowseLibraryInput{
					ItemTypes: "Movie",
					Limit:     1,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(browseOut.Items).NotTo(BeEmpty())
				itemID := browseOut.Items[0].ID

				// Mark played
				_, playedOut, err := SetPlayed(context.Background(), nil, SetPlayedInput{
					ItemID: itemID,
					Played: true,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(playedOut.Success).To(BeTrue())

				// Mark unplayed
				_, unplayedOut, err := SetPlayed(context.Background(), nil, SetPlayedInput{
					ItemID: itemID,
					Played: false,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(unplayedOut.Success).To(BeTrue())
			})
		})
	})

	Context("User-scoped operations without user ID", func() {
		It("should return an error for get_latest without user ID", func() {
			savedUserID := client.UserID
			client.UserID = ""
			defer func() { client.UserID = savedUserID }()

			_, _, err := GetLatest(context.Background(), nil, GetLatestInput{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("JELLYFIN_USER_ID"))
		})

		It("should return an error for get_next_up without user ID", func() {
			savedUserID := client.UserID
			client.UserID = ""
			defer func() { client.UserID = savedUserID }()

			_, _, err := GetNextUp(context.Background(), nil, GetNextUpInput{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("JELLYFIN_USER_ID"))
		})

		It("should return an error for set_favorite without user ID", func() {
			savedUserID := client.UserID
			client.UserID = ""
			defer func() { client.UserID = savedUserID }()

			_, _, err := SetFavorite(context.Background(), nil, SetFavoriteInput{
				ItemID:   "some-id",
				Favorite: true,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("JELLYFIN_USER_ID"))
		})

		It("should return an error for set_played without user ID", func() {
			savedUserID := client.UserID
			client.UserID = ""
			defer func() { client.UserID = savedUserID }()

			_, _, err := SetPlayed(context.Background(), nil, SetPlayedInput{
				ItemID: "some-id",
				Played: true,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("JELLYFIN_USER_ID"))
		})
	})
})
