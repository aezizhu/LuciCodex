package testutil

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestAssertHelpers(t *testing.T) {
	// We can only test success paths easily without mocking testing.T
	AssertNoError(t, nil)
	AssertError(t, errors.New("error"))
	AssertEqual(t, 1, 1)
	AssertContains(t, "hello world", "world")
	AssertNotContains(t, "hello world", "foo")
	AssertTrue(t, true)
	AssertFalse(t, false)
}

func TestTempConfig(t *testing.T) {
	cfg := DefaultTestConfig()
	path := TempConfig(t, cfg)
	if !FileExists(path) {
		t.Error("TempConfig file not created")
	}
	// Verify content
	// ...
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
	path := TempFile(t, "content")
	if !FileExists(path) {
		t.Error("TempFile not created")
	}
	content, err := os.ReadFile(path)
	AssertNoError(t, err)
	AssertEqual(t, string(content), "content")
}

func TestTempDir(t *testing.T) {
	dir := TempDir(t)
	if !DirExists(dir) {
		t.Error("TempDir not created")
	}
}

func TestFileExists(t *testing.T) {
	tmp := TempFile(t, "")
	AssertTrue(t, FileExists(tmp))
	AssertFalse(t, FileExists(tmp+"_nonexistent"))
	AssertFalse(t, FileExists(filepath.Dir(tmp))) // Dir is not file
}

func TestDirExists(t *testing.T) {
	tmp := TempDir(t)
	AssertTrue(t, DirExists(tmp))
	AssertFalse(t, DirExists(tmp+"_nonexistent"))
	// File is not dir
	f := TempFile(t, "")
	AssertFalse(t, DirExists(f))
}

func TestStripAnsi(t *testing.T) {
	colored := "\033[31mRed\033[0m"
	stripped := StripAnsi(colored)
	AssertEqual(t, stripped, "Red")
}

// MockTestingT is a mock implementation of TestingT
type MockTestingT struct {
	Failed bool
	Msg    string
}

func (m *MockTestingT) Helper() {}
func (m *MockTestingT) Fatalf(format string, args ...interface{}) {
	m.Failed = true
	m.Msg = format // simplified
}
func (m *MockTestingT) Fatal(args ...interface{}) {
	m.Failed = true
	m.Msg = "fatal"
}
func (m *MockTestingT) Errorf(format string, args ...interface{}) {
	m.Failed = true
	m.Msg = format
}
func (m *MockTestingT) Error(args ...interface{}) {
	m.Failed = true
	m.Msg = "error"
}
func (m *MockTestingT) TempDir() string {
	return os.TempDir()
}
func (m *MockTestingT) Logf(format string, args ...interface{}) {}

func TestAssertHelpers_Failures(t *testing.T) {
	mock := &MockTestingT{}

	AssertNoError(mock, errors.New("error"))
	if !mock.Failed {
		t.Error("AssertNoError should fail on error")
	}
	mock.Failed = false

	AssertError(mock, nil)
	if !mock.Failed {
		t.Error("AssertError should fail on nil")
	}
	mock.Failed = false

	AssertEqual(mock, 1, 2)
	if !mock.Failed {
		t.Error("AssertEqual should fail on inequality")
	}
	mock.Failed = false

	AssertContains(mock, "hello", "world")
	if !mock.Failed {
		t.Error("AssertContains should fail on missing substring")
	}
	mock.Failed = false

	AssertNotContains(mock, "hello world", "world")
	if !mock.Failed {
		t.Error("AssertNotContains should fail on present substring")
	}
	mock.Failed = false

	AssertTrue(mock, false)
	if !mock.Failed {
		t.Error("AssertTrue should fail on false")
	}
	mock.Failed = false

	AssertFalse(mock, true)
	if !mock.Failed {
		t.Error("AssertFalse should fail on true")
	}
}

func TestMockHTTPServerJSON_Error(t *testing.T) {
	// Pass a channel which cannot be marshaled to JSON
	server := MockHTTPServerJSON(t, 200, make(chan int))
	defer server.Close()

	resp, err := http.Get(server.URL)
	AssertNoError(t, err)
	defer resp.Body.Close()

	// Server logs error but returns 200 (and partial/empty body)
	// We just want to ensure it doesn't panic and hits the error path
}
