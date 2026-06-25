package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"git.k3nny.fr/glint/internal/fetcher"
)

// frame encodes v as a Content-Length–framed LSP message.
func frame(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	hdr := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	return append([]byte(hdr), body...)
}

// readMsg reads one Content-Length–framed JSON object from r.
func readMsg(t *testing.T, r *bufio.Reader) map[string]json.RawMessage {
	t.Helper()
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			n, err := strconv.Atoi(strings.TrimPrefix(line, "Content-Length: "))
			if err != nil {
				t.Fatalf("invalid Content-Length: %v", err)
			}
			contentLength = n
		}
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("reading body: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

// newTestServer returns a Server with a captured exit code and a bufio.Reader
// wrapping the output buffer so tests can read back server messages.
func newTestServer(input []byte) (*Server, *bytes.Buffer, *int) {
	var out bytes.Buffer
	exitCode := -1
	srv := New(bytes.NewReader(input), &out, fetcher.GitLabConfig{}, "test")
	srv.Exit = func(code int) { exitCode = code }
	return srv, &out, &exitCode
}

func TestServer_Initialize(t *testing.T) {
	input := frame(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	})
	srv, out, _ := newTestServer(input)
	srv.Run() //nolint:errcheck

	resp := readMsg(t, bufio.NewReader(out))
	if string(resp["id"]) != "1" {
		t.Errorf("response id = %s; want 1", resp["id"])
	}

	var result InitializeResult
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Capabilities.TextDocumentSync != 1 {
		t.Errorf("textDocumentSync = %d; want 1", result.Capabilities.TextDocumentSync)
	}
	if result.ServerInfo == nil || result.ServerInfo.Name != "glint" {
		t.Errorf("serverInfo.name = %v; want glint", result.ServerInfo)
	}
}

func TestServer_ShutdownExit(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(frame(t, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{},
	}))
	buf.Write(frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{},
	}))
	buf.Write(frame(t, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "shutdown",
	}))
	buf.Write(frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "exit",
	}))

	srv, out, exitCode := newTestServer(buf.Bytes())
	srv.Run() //nolint:errcheck

	r := bufio.NewReader(out)
	initResp := readMsg(t, r)
	if string(initResp["id"]) != "1" {
		t.Errorf("init response id = %s; want 1", initResp["id"])
	}
	shutResp := readMsg(t, r)
	if string(shutResp["id"]) != "2" {
		t.Errorf("shutdown response id = %s; want 2", shutResp["id"])
	}
	if string(shutResp["result"]) != "null" {
		t.Errorf("shutdown result = %s; want null", shutResp["result"])
	}
	if *exitCode != 0 {
		t.Errorf("exit code = %d; want 0", *exitCode)
	}
}

func TestServer_ExitWithoutShutdown(t *testing.T) {
	input := frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "exit",
	})
	srv, _, exitCode := newTestServer(input)
	srv.Run() //nolint:errcheck

	if *exitCode != 1 {
		t.Errorf("exit code = %d; want 1 (no prior shutdown)", *exitCode)
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	input := frame(t, map[string]any{
		"jsonrpc": "2.0", "id": 99, "method": "workspace/unknownMethod",
	})
	srv, out, _ := newTestServer(input)
	srv.Run() //nolint:errcheck

	resp := readMsg(t, bufio.NewReader(out))
	if resp["error"] == nil {
		t.Errorf("expected error response for unknown method, got: %v", resp)
	}
	var rpcErr RPCError
	if err := json.Unmarshal(resp["error"], &rpcErr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if rpcErr.Code != -32601 {
		t.Errorf("error code = %d; want -32601", rpcErr.Code)
	}
}

func TestServer_UnknownNotificationIgnored(t *testing.T) {
	// Notifications (no id) for unknown methods must be silently ignored.
	input := frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "$/setTrace", "params": map[string]any{"value": "off"},
	})
	srv, out, _ := newTestServer(input)
	srv.Run() //nolint:errcheck

	if out.Len() > 0 {
		t.Errorf("server wrote %d bytes for unknown notification; want 0", out.Len())
	}
}

func TestServer_DidOpen_CleanPipeline(t *testing.T) {
	yaml := `stages: [build]

build-job:
  stage: build
  script: echo hello
`
	// Use a pseudo file:// URI that maps to the tmp path; include resolution
	// will fail silently (no network, no local includes) which is fine.
	uri := "file:///tmp/test.gitlab-ci.yml"

	input := frame(t, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri": uri, "languageId": "yaml", "version": 1, "text": yaml,
			},
		},
	})
	srv, out, _ := newTestServer(input)
	srv.Run() //nolint:errcheck

	notif := readMsg(t, bufio.NewReader(out))
	if string(notif["method"]) != `"textDocument/publishDiagnostics"` {
		t.Fatalf("method = %s; want textDocument/publishDiagnostics", notif["method"])
	}
	var params PublishDiagnosticsParams
	if err := json.Unmarshal(notif["params"], &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params.URI != uri {
		t.Errorf("uri = %q; want %q", params.URI, uri)
	}
	// A clean pipeline should produce no diagnostics (or only warnings from
	// include resolution being skipped — but those are filtered since they
	// originate from a different file path).
	for _, d := range params.Diagnostics {
		if d.Severity == 1 {
			t.Errorf("unexpected error diagnostic: %s", d.Message)
		}
	}
}

func TestServer_DidOpen_WithErrors(t *testing.T) {
	// A pipeline with a job in an undeclared stage triggers GL004.
	yaml := `stages: [build]

bad-job:
  stage: missing-stage
  script: echo hi
`
	uri := "file:///tmp/bad.gitlab-ci.yml"
	input := frame(t, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri": uri, "languageId": "yaml", "version": 1, "text": yaml,
			},
		},
	})
	srv, out, _ := newTestServer(input)
	srv.Run() //nolint:errcheck

	notif := readMsg(t, bufio.NewReader(out))
	var params PublishDiagnosticsParams
	if err := json.Unmarshal(notif["params"], &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if len(params.Diagnostics) == 0 {
		t.Error("expected diagnostics for pipeline with unknown stage, got none")
	}
	found := false
	for _, d := range params.Diagnostics {
		if d.Code == "GL004" {
			found = true
			if d.Severity != 1 {
				t.Errorf("GL004 severity = %d; want 1 (Error)", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected GL004 diagnostic, got: %v", params.Diagnostics)
	}
}

func TestServer_DidOpen_ParseError(t *testing.T) {
	uri := "file:///tmp/broken.gitlab-ci.yml"
	input := frame(t, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri": uri, "languageId": "yaml", "version": 1,
				"text": "?", // bare ? yields empty job name → parse error
			},
		},
	})
	srv, out, _ := newTestServer(input)
	srv.Run() //nolint:errcheck

	notif := readMsg(t, bufio.NewReader(out))
	var params PublishDiagnosticsParams
	if err := json.Unmarshal(notif["params"], &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if len(params.Diagnostics) == 0 {
		t.Fatal("expected parse-error diagnostic, got none")
	}
	d := params.Diagnostics[0]
	if d.Severity != 1 {
		t.Errorf("severity = %d; want 1 (Error)", d.Severity)
	}
	if !strings.Contains(d.Message, "YAML parse error") {
		t.Errorf("message = %q; want YAML parse error", d.Message)
	}
}

func TestServer_DidChange(t *testing.T) {
	uri := "file:///tmp/ci.gitlab-ci.yml"
	var buf bytes.Buffer
	// Open with clean content.
	buf.Write(frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "yaml", "version": 1,
			"text": "stages: [build]\nbuild: {stage: build, script: echo}\n",
		}},
	}))
	// Change to content with an error.
	buf.Write(frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didChange",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": uri, "version": 2},
			"contentChanges": []map[string]any{
				{"text": "stages: [build]\nbad: {stage: gone, script: hi}\n"},
			},
		},
	}))

	srv, out, _ := newTestServer(buf.Bytes())
	srv.Run() //nolint:errcheck

	r := bufio.NewReader(out)
	_ = readMsg(t, r) // first publishDiagnostics (clean)
	second := readMsg(t, r)

	var params PublishDiagnosticsParams
	if err := json.Unmarshal(second["params"], &params); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(params.Diagnostics) == 0 {
		t.Error("expected diagnostics after change to broken content, got none")
	}
}

func TestServer_DidClose_ClearsdiAgnostics(t *testing.T) {
	uri := "file:///tmp/toclose.gitlab-ci.yml"
	var buf bytes.Buffer
	buf.Write(frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "yaml", "version": 1,
			"text": "stages: [build]\nj: {stage: build, script: echo}\n",
		}},
	}))
	buf.Write(frame(t, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didClose",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri}},
	}))

	srv, out, _ := newTestServer(buf.Bytes())
	srv.Run() //nolint:errcheck

	r := bufio.NewReader(out)
	_ = readMsg(t, r) // publishDiagnostics from didOpen

	closeNotif := readMsg(t, r)
	var params PublishDiagnosticsParams
	if err := json.Unmarshal(closeNotif["params"], &params); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if params.URI != uri {
		t.Errorf("uri = %q; want %q", params.URI, uri)
	}
	if len(params.Diagnostics) != 0 {
		t.Errorf("expected empty diagnostics on close, got %v", params.Diagnostics)
	}
}

func TestServer_UriToPath(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"file:///tmp/ci.yml", "/tmp/ci.yml"},
		{"file:///home/user/project/.gitlab-ci.yml", "/home/user/project/.gitlab-ci.yml"},
		{"https://example.com/file.yml", ""},
		{"not-a-uri", ""},
	}
	for _, tc := range tests {
		got := uriToPath(tc.uri)
		if got != tc.want {
			t.Errorf("uriToPath(%q) = %q; want %q", tc.uri, got, tc.want)
		}
	}
}
