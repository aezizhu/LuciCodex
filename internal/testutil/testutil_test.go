package testutil

import (
	"errors"
	"net/http"
	"os"
	"testing"
)

func TestAssertNoError(t *testing.T) {
	// Should not fail
	AssertNoError(t, nil)

	// We can't easily test that it fails without mocking testing.T,
	// but we can verify it compiles and runs for success case.
}

func TestAssertError(t *testing.T) {
	AssertError(t, errors.New("test error"))
}

func TestAssertEqual(t *testing.T) {
	AssertEqual(t, 1, 1)
	AssertEqual(t, "a", "a")
}

func TestAssertContains(t *testing.T) {
	AssertContains(t, "hello world", "world")
}

func TestAssertNotContains(t *testing.T) {
	AssertNotContains(t, "hello world", "foo")
}

func TestAssertTrue(t *testing.T) {
	AssertTrue(t, true)
}

func TestAssertFalse(t *testing.T) {
	AssertFalse(t, false)
}

func TestTempConfig(t *testing.T) {
	cfg := DefaultTestConfig()
	path := TempConfig(t, cfg)
	defer os.Remove(path)

	if !FileExists(path) {
		t.Error("TempConfig failed to create file")
	}
}

func TestMockHTTPServer(t *testing.T) {
	server := MockHTTPServer(t, 200, "response")
	defer server.Close()

	resp, err := http.Get(server.URL)
	AssertNoError(t, err)
	defer resp.Body.Close()

	body := ReadBody(t, resp.Body)
	AssertEqual(t, body, "response")
}

func TestMockHTTPServerJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	server := MockHTTPServerJSON(t, 200, data)
	defer server.Close()

	resp, err := http.Get(server.URL)
	AssertNoError(t, err)
	defer resp.Body.Close()

	body := ReadBody(t, resp.Body)
	AssertContains(t, body, `"key":"value"`)
}

func TestMockHTTPServerFunc(t *testing.T) {
	server := MockHTTPServerFunc(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		w.Write([]byte("custom"))
	})
	defer server.Close()

	resp, err := http.Get(server.URL)
	AssertNoError(t, err)
	defer resp.Body.Close()

	AssertEqual(t, resp.StatusCode, 201)
	body := ReadBody(t, resp.Body)
	AssertEqual(t, body, "custom")
}

func TestTempFile(t *testing.T) {
	content := "test content"
	path := TempFile(t, content)
	defer os.Remove(path)

	if !FileExists(path) {
		t.Error("TempFile failed to create file")
	}

	readContent, err := os.ReadFile(path)
	AssertNoError(t, err)
	AssertEqual(t, string(readContent), content)
}

func TestDirExists(t *testing.T) {
	dir := TempDir(t)
	// TempDir is automatically cleaned up by t.Cleanup?
	// t.TempDir() returns a dir that is cleaned up.

	if !DirExists(dir) {
		t.Error("DirExists returned false for existing dir")
	}

	if DirExists("/non/existent/dir") {
		t.Error("DirExists returned true for non-existent dir")
	}
}

func TestStripAnsi(t *testing.T) {
	colored := "\033[31mred\033[0m"
	plain := StripAnsi(colored)
	AssertEqual(t, plain, "red")
}
