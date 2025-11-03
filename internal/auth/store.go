package auth

import (
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "sync"
    "time"
)

type Token struct {
    Provider     string    `json:"provider"`
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token,omitempty"`
    TokenType    string    `json:"token_type,omitempty"`
    Expiry       time.Time `json:"expiry,omitempty"`
    Scope        string    `json:"scope,omitempty"`
}

type Store struct {
    path string
    mu   sync.Mutex
    // keyed by provider name
    tokens map[string]Token
}

func NewStore(path string) *Store {
    return &Store{path: path, tokens: map[string]Token{}}
}

func defaultPath() string {
    home, _ := os.UserHomeDir()
    if home != "" {
        return filepath.Join(home, ".config", "lucicodex", "tokens.json")
    }
    return "/etc/lucicodex/tokens.json"
}

func (s *Store) PathOrDefault() string {
    if s.path != "" {
        return s.path
    }
    return defaultPath()
}

func (s *Store) Load() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    p := s.PathOrDefault()
    b, err := os.ReadFile(p)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            s.tokens = map[string]Token{}
            return nil
        }
        return err
    }
    return json.Unmarshal(b, &s.tokens)
}

func (s *Store) Save() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    p := s.PathOrDefault()
    if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
        return err
    }
    b, _ := json.MarshalIndent(s.tokens, "", "  ")
    f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = f.Write(b)
    return err
}

func (s *Store) Get(provider string) (Token, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    t, ok := s.tokens[provider]
    return t, ok
}

func (s *Store) Put(t Token) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.tokens == nil { s.tokens = map[string]Token{} }
    s.tokens[t.Provider] = t
}

func (s *Store) Delete(provider string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.tokens, provider)
}


