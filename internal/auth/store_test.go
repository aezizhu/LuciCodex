package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStore_PutGetDelete(t *testing.T) {
	store := NewStore("")

	// Test Put and Get
	token := Token{
		Provider:    "test-provider",
		AccessToken: "test-access-token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	store.Put(token)

	retrievedToken, ok := store.Get("test-provider")
	if !ok {
		t.Fatal("expected to find token for 'test-provider', but didn't")
	}
	if retrievedToken.AccessToken != "test-access-token" {
		t.Errorf("expected access token '%s', got '%s'", "test-access-token", retrievedToken.AccessToken)
	}

	// Test Delete
	store.Delete("test-provider")
	_, ok = store.Get("test-provider")
	if ok {
		t.Fatal("expected token to be deleted, but it was found")
	}
}

func TestStore_SaveLoad(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "tokens.json")

	// Create and save a store
	store1 := NewStore(tokenFile)
	token1 := Token{Provider: "provider1", AccessToken: "token1"}
	token2 := Token{Provider: "provider2", AccessToken: "token2"}
	store1.Put(token1)
	store1.Put(token2)

	err := store1.Save()
	if err != nil {
		t.Fatalf("failed to save token store: %v", err)
	}

	// Create a new store and load from the file
	store2 := NewStore(tokenFile)
	err = store2.Load()
	if err != nil {
		t.Fatalf("failed to load token store: %v", err)
	}

	// Verify content
	retrievedToken1, ok := store2.Get("provider1")
	if !ok || retrievedToken1.AccessToken != "token1" {
		t.Error("failed to retrieve correct token for 'provider1' after loading")
	}

	retrievedToken2, ok := store2.Get("provider2")
	if !ok || retrievedToken2.AccessToken != "token2" {
		t.Error("failed to retrieve correct token for 'provider2' after loading")
	}
}

func TestStore_Load_NotExist(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "non-existent-tokens.json")

	store := NewStore(tokenFile)
	err := store.Load()
	if err != nil {
		t.Fatalf("loading a non-existent file should not produce an error, but got: %v", err)
	}

	if len(store.tokens) != 0 {
		t.Errorf("expected empty token map after loading non-existent file, but got %d tokens", len(store.tokens))
	}
}

func TestStore_PathOrDefault(t *testing.T) {
	// Test with a specific path
	storeWithPath := NewStore("/custom/path/tokens.json")
	if storeWithPath.PathOrDefault() != "/custom/path/tokens.json" {
		t.Errorf("expected custom path, but got %s", storeWithPath.PathOrDefault())
	}

	// Test with default path
	storeDefault := NewStore("")
	defaultPath := storeDefault.PathOrDefault()
	if defaultPath == "" {
		t.Fatal("default path should not be empty")
	}
	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, ".config", "lucicodex", "tokens.json")
	if home == "" {
		expectedPath = "/etc/lucicodex/tokens.json"
	}
	if defaultPath != expectedPath {
		t.Errorf("expected default path '%s', but got '%s'", expectedPath, defaultPath)
	}
}

func TestStore_Concurrency(t *testing.T) {
	store := NewStore("")
	var wg sync.WaitGroup
	numRoutines := 100

	// Concurrently write
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			provider := fmt.Sprintf("provider-%d", i)
			token := Token{Provider: provider, AccessToken: "some-token"}
			store.Put(token)
		}(i)
	}

	// Concurrently read
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			provider := fmt.Sprintf("provider-%d", i)
			store.Get(provider)
		}(i)
	}

	wg.Wait()

	// This test mainly checks for race conditions.
	// The race detector should be run with `go test -race`.
	// We can do a final check to see if the map size is correct.
	if len(store.tokens) != numRoutines {
		t.Errorf("expected %d tokens in the store, but got %d", numRoutines, len(store.tokens))
	}
}

func TestStore_PathOrDefault_NoHome(t *testing.T) {
	t.Setenv("HOME", "")
	// On some systems, UserHomeDir might still return something from /etc/passwd even if HOME is empty.
	// But if it returns empty, we expect /etc/lucicodex/tokens.json

	// We can't easily mock os.UserHomeDir without refactoring.
	// But we can check if it returns empty.
	home, _ := os.UserHomeDir()
	if home == "" {
		s := NewStore("")
		if s.PathOrDefault() != "/etc/lucicodex/tokens.json" {
			t.Errorf("expected /etc/lucicodex/tokens.json when HOME is empty")
		}
	}
}

func TestStore_Save_Errors(t *testing.T) {
	// Test MkdirAll error (permission denied)
	// We can try to save to a root path
	s := NewStore("/root/tokens.json")
	if os.Geteuid() != 0 { // Only if not root
		err := s.Save()
		if err == nil {
			// It might fail or not depending on env, but usually fails
		}
	}

	// Better: save to a file where directory is a file
	tempDir := t.TempDir()
	fileAsDir := filepath.Join(tempDir, "file")
	os.WriteFile(fileAsDir, []byte("test"), 0600)

	s = NewStore(filepath.Join(fileAsDir, "tokens.json"))
	err := s.Save()
	if err == nil {
		t.Error("expected error when directory is a file")
	}
}

func TestStore_Put_NilMap(t *testing.T) {
	// Manually create store with nil map
	s := &Store{}
	s.Put(Token{Provider: "test"})

	if len(s.tokens) != 1 {
		t.Error("expected token to be added even if map was nil")
	}
}
