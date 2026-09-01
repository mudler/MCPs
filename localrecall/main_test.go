package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func useLocalRecallHTTPServer(t *testing.T, handler http.Handler) {
	t.Helper()

	server := httptest.NewServer(handler)
	previousClient := httpClient
	previousURL := localRecallURL
	httpClient = server.Client()
	localRecallURL = server.URL
	t.Cleanup(func() {
		server.Close()
		httpClient = previousClient
		localRecallURL = previousURL
	})
}

func TestListFilesPreservesEntryKeys(t *testing.T) {
	useLocalRecallHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/collections/project/entries" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/api/collections/project/entries")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"collection": "project",
				"entries": ["memory.md"],
				"keys": ["70fb48f6/memory.md"],
				"count": 1
			}
		}`))
	}))

	_, output, err := ListFiles(context.Background(), nil, ListFilesInput{CollectionName: "project"})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	want := []string{"70fb48f6/memory.md"}
	if !reflect.DeepEqual(output.Keys, want) {
		t.Errorf("ListFiles().Keys = %#v, want %#v", output.Keys, want)
	}
}

func TestGetEntryReturnsContentAndChunkCount(t *testing.T) {
	const content = "Revision-ID: rev-42\nbody"

	useLocalRecallHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		const wantPath = "/api/collections/project%20castrum/entries/70fb48f6%2Fmemory.md"
		if r.URL.EscapedPath() != wantPath {
			t.Errorf("escaped path = %q, want %q", r.URL.EscapedPath(), wantPath)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"collection": "project castrum",
				"entry": "70fb48f6/memory.md",
				"content": "Revision-ID: rev-42\nbody",
				"chunk_count": 3
			}
		}`))
	}))

	_, output, err := GetEntry(context.Background(), nil, GetEntryInput{
		CollectionName:  "project castrum",
		Entry:           "70fb48f6/memory.md",
		MaxContentChars: 4_000,
	})
	if err != nil {
		t.Fatalf("GetEntry() error = %v", err)
	}

	want := GetEntryOutput{
		Collection:       "project castrum",
		Entry:            "70fb48f6/memory.md",
		Content:          content,
		ChunkCount:       3,
		ContentLength:    24,
		ContentTruncated: false,
	}
	if !reflect.DeepEqual(output, want) {
		t.Errorf("GetEntry() = %#v, want %#v", output, want)
	}
}

func TestGetEntryTruncatesContentByUnicodeCharacters(t *testing.T) {
	useLocalRecallHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"collection": "project",
				"entry": "memory.md",
				"content": "é😊abc",
				"chunk_count": 2
			}
		}`))
	}))

	_, output, err := GetEntry(context.Background(), nil, GetEntryInput{
		CollectionName:  "project",
		Entry:           "memory.md",
		MaxContentChars: 2,
	})
	if err != nil {
		t.Fatalf("GetEntry() error = %v", err)
	}

	if output.Content != "é😊" {
		t.Errorf("GetEntry().Content = %q, want %q", output.Content, "é😊")
	}
	if output.ContentLength != 5 {
		t.Errorf("GetEntry().ContentLength = %d, want 5", output.ContentLength)
	}
	if !output.ContentTruncated {
		t.Error("GetEntry().ContentTruncated = false, want true")
	}
}

func TestGetEntryDefaultsToFourThousandContentCharacters(t *testing.T) {
	content := strings.Repeat("x", 4_001)
	useLocalRecallHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"collection":  "project",
				"entry":       "memory.md",
				"content":     content,
				"chunk_count": 3,
			},
		})
	}))

	_, output, err := GetEntry(context.Background(), nil, GetEntryInput{
		CollectionName: "project",
		Entry:          "memory.md",
	})
	if err != nil {
		t.Fatalf("GetEntry() error = %v", err)
	}

	if got := len([]rune(output.Content)); got != 4_000 {
		t.Errorf("len(GetEntry().Content) = %d, want 4000", got)
	}
	if output.ContentLength != 4_001 {
		t.Errorf("GetEntry().ContentLength = %d, want 4001", output.ContentLength)
	}
	if !output.ContentTruncated {
		t.Error("GetEntry().ContentTruncated = false, want true")
	}
}

func TestGetEntryCapsRequestedContentCharactersAtOneHundredThousand(t *testing.T) {
	content := strings.Repeat("x", 100_001)
	useLocalRecallHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"collection":  "project",
				"entry":       "memory.md",
				"content":     content,
				"chunk_count": 50,
			},
		})
	}))

	_, output, err := GetEntry(context.Background(), nil, GetEntryInput{
		CollectionName:  "project",
		Entry:           "memory.md",
		MaxContentChars: 200_000,
	})
	if err != nil {
		t.Fatalf("GetEntry() error = %v", err)
	}

	if got := len([]rune(output.Content)); got != 100_000 {
		t.Errorf("len(GetEntry().Content) = %d, want 100000", got)
	}
	if output.ContentLength != 100_001 {
		t.Errorf("GetEntry().ContentLength = %d, want 100001", output.ContentLength)
	}
	if !output.ContentTruncated {
		t.Error("GetEntry().ContentTruncated = false, want true")
	}
}

func TestGetEntryRejectsNegativeContentLimit(t *testing.T) {
	useLocalRecallHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"collection": "project",
				"entry": "memory.md",
				"content": "content",
				"chunk_count": 1
			}
		}`))
	}))

	_, _, err := GetEntry(context.Background(), nil, GetEntryInput{
		CollectionName:  "project",
		Entry:           "memory.md",
		MaxContentChars: -1,
	})
	if err == nil {
		t.Fatal("GetEntry() error = nil, want a validation error")
	}
}

func TestGetEntryWithoutCollectionUsesConfiguredCollection(t *testing.T) {
	useLocalRecallHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const wantPath = "/api/collections/project%20castrum/entries/memory.md"
		if r.URL.EscapedPath() != wantPath {
			t.Errorf("escaped path = %q, want %q", r.URL.EscapedPath(), wantPath)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"collection": "project castrum",
				"entry": "memory.md",
				"content": "Revision-ID: rev-42",
				"chunk_count": 3
			}
		}`))
	}))

	previousCollection := defaultCollectionName
	defaultCollectionName = "project castrum"
	t.Cleanup(func() {
		defaultCollectionName = previousCollection
	})

	_, output, err := GetEntryWithoutCollection(context.Background(), nil, GetEntryInputWithoutCollection{
		Entry:           "memory.md",
		MaxContentChars: 4_000,
	})
	if err != nil {
		t.Fatalf("GetEntryWithoutCollection() error = %v", err)
	}
	if output.Collection != "project castrum" {
		t.Errorf("GetEntryWithoutCollection().Collection = %q, want %q", output.Collection, "project castrum")
	}
	if output.ChunkCount != 3 {
		t.Errorf("GetEntryWithoutCollection().ChunkCount = %d, want 3", output.ChunkCount)
	}
}

func TestNewServerRegistersGetEntryWithTheConfiguredCollectionSchema(t *testing.T) {
	previousClient := httpClient
	previousURL := localRecallURL
	previousAPIKey := apiKey
	previousCollection := defaultCollectionName
	previousDebugMode := debugMode
	t.Cleanup(func() {
		httpClient = previousClient
		localRecallURL = previousURL
		apiKey = previousAPIKey
		defaultCollectionName = previousCollection
		debugMode = previousDebugMode
	})

	tests := []struct {
		name               string
		defaultCollection  string
		wantCollectionName bool
	}{
		{
			name:               "explicit collection",
			wantCollectionName: true,
		},
		{
			name:              "default collection",
			defaultCollection: "project",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LOCALRECALL_ENABLED_TOOLS", "get_entry")
			t.Setenv("LOCALRECALL_COLLECTION", test.defaultCollection)

			tools := listedServerTools(t, newServer())
			if len(tools) != 1 {
				t.Fatalf("len(ListTools().Tools) = %d, want 1", len(tools))
			}
			if tools[0].Name != "get_entry" {
				t.Fatalf("ListTools().Tools[0].Name = %q, want %q", tools[0].Name, "get_entry")
			}

			schema, ok := tools[0].InputSchema.(map[string]interface{})
			if !ok {
				t.Fatalf("get_entry input schema has type %T, want map[string]interface{}", tools[0].InputSchema)
			}
			properties, ok := schema["properties"].(map[string]interface{})
			if !ok {
				t.Fatalf("get_entry input schema properties have type %T, want map[string]interface{}", schema["properties"])
			}

			_, hasCollectionName := properties["collection_name"]
			if hasCollectionName != test.wantCollectionName {
				t.Errorf("get_entry schema has collection_name = %t, want %t", hasCollectionName, test.wantCollectionName)
			}
			for _, name := range []string{"entry", "max_content_chars"} {
				if _, ok := properties[name]; !ok {
					t.Errorf("get_entry schema is missing %q", name)
				}
			}
		})
	}
}

func listedServerTools(t *testing.T, server *mcp.Server) []*mcp.Tool {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	return result.Tools
}
