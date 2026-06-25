package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/linter"
	"git.k3nny.fr/glint/internal/model"
	"git.k3nny.fr/glint/internal/resolver"
)

// Server is a minimal Language Server Protocol server that publishes glint
// diagnostics for .gitlab-ci.yml files opened in an editor.
//
// Transport: JSON-RPC 2.0 over stdin/stdout with Content-Length framing.
// Sync mode: Full — the client sends the complete document text on every change.
type Server struct {
	in      *bufio.Reader
	out     io.Writer
	cfg     fetcher.GitLabConfig
	version string
	docs    map[string]string // uri → current document text

	// Exit is called with the process exit code when the LSP client sends the
	// "exit" notification. Defaults to os.Exit; replace in tests.
	Exit func(int)

	shutdownReceived bool
}

// New creates a Server reading from r and writing to w.
func New(r io.Reader, w io.Writer, cfg fetcher.GitLabConfig, version string) *Server {
	return &Server{
		in:      bufio.NewReader(r),
		out:     w,
		cfg:     cfg,
		version: version,
		docs:    make(map[string]string),
		Exit:    os.Exit,
	}
}

// Run processes LSP messages until the connection closes or a fatal error occurs.
// It returns nil on a clean EOF (client disconnected) and a non-nil error for
// unrecoverable protocol failures.
func (s *Server) Run() error {
	for {
		msg, err := s.readMessage()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading LSP message: %w", err)
		}
		if err := s.dispatch(msg); err != nil {
			return err
		}
	}
}

// readMessage reads one Content-Length–framed JSON-RPC message from the stream.
func (s *Server) readMessage() (*Message, error) {
	var contentLength int
	for {
		line, err := s.in.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // blank line separates headers from body
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			n, parseErr := strconv.Atoi(strings.TrimPrefix(line, "Content-Length: "))
			if parseErr != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", parseErr)
			}
			contentLength = n
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("missing or zero Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(s.in, body); err != nil {
		return nil, fmt.Errorf("reading message body: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("unmarshalling message: %w", err)
	}
	return &msg, nil
}

// writeMessage encodes v as JSON and sends it with a Content-Length header.
func (s *Server) writeMessage(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(s.out, header); err != nil {
		return err
	}
	_, err = s.out.Write(body)
	return err
}

func (s *Server) respond(id json.RawMessage, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.writeMessage(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
	}{"2.0", id, raw})
}

func (s *Server) respondError(id json.RawMessage, code int, message string) error {
	return s.writeMessage(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   RPCError        `json:"error"`
	}{"2.0", id, RPCError{Code: code, Message: message}})
}

func (s *Server) notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return s.writeMessage(struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}{"2.0", method, raw})
}

// isRequest reports whether msg is a JSON-RPC request (has a non-null id).
func isRequest(msg *Message) bool {
	return len(msg.ID) > 0 && string(msg.ID) != "null"
}

func (s *Server) dispatch(msg *Message) error {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "initialized":
		return nil // notification; no response required
	case "shutdown":
		s.shutdownReceived = true
		if isRequest(msg) {
			return s.respond(msg.ID, nil)
		}
		return nil
	case "exit":
		code := 1
		if s.shutdownReceived {
			code = 0
		}
		s.Exit(code)
		return nil
	case "textDocument/didOpen":
		return s.handleDidOpen(msg)
	case "textDocument/didChange":
		return s.handleDidChange(msg)
	case "textDocument/didSave":
		return s.handleDidSave(msg)
	case "textDocument/didClose":
		return s.handleDidClose(msg)
	default:
		if isRequest(msg) {
			return s.respondError(msg.ID, -32601, "method not found: "+msg.Method)
		}
		return nil
	}
}

func (s *Server) handleInitialize(msg *Message) error {
	return s.respond(msg.ID, InitializeResult{
		Capabilities: ServerCapabilities{TextDocumentSync: 1},
		ServerInfo:   &ServerInfo{Name: "glint", Version: s.version},
	})
}

func (s *Server) handleDidOpen(msg *Message) error {
	var p DidOpenTextDocumentParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return nil // ignore malformed notifications
	}
	s.docs[p.TextDocument.URI] = p.TextDocument.Text
	return s.lintAndPublish(p.TextDocument.URI, p.TextDocument.Text)
}

func (s *Server) handleDidChange(msg *Message) error {
	var p DidChangeTextDocumentParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return nil
	}
	if len(p.ContentChanges) == 0 {
		return nil
	}
	// Full sync: the last change event holds the complete new text.
	text := p.ContentChanges[len(p.ContentChanges)-1].Text
	s.docs[p.TextDocument.URI] = text
	return s.lintAndPublish(p.TextDocument.URI, text)
}

func (s *Server) handleDidSave(msg *Message) error {
	var p DidSaveTextDocumentParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return nil
	}
	text := s.docs[p.TextDocument.URI]
	if p.Text != nil {
		text = *p.Text
		s.docs[p.TextDocument.URI] = text
	}
	if text == "" {
		return nil
	}
	return s.lintAndPublish(p.TextDocument.URI, text)
}

func (s *Server) handleDidClose(msg *Message) error {
	var p DidCloseTextDocumentParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return nil
	}
	delete(s.docs, p.TextDocument.URI)
	// Clear diagnostics so the editor doesn't show stale squiggles.
	return s.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         p.TextDocument.URI,
		Diagnostics: []Diagnostic{},
	})
}

func (s *Server) lintAndPublish(uri, text string) error {
	diags := s.lintDocument(uri, text)
	return s.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})
}

// lintDocument parses text and runs all lint rules, returning LSP Diagnostics.
// Findings that originate from included files (not the root document) are
// excluded; their URIs are not tracked so line numbers would be incorrect.
func (s *Server) lintDocument(uri, text string) []Diagnostic {
	path := uriToPath(uri)
	if path == "" {
		return []Diagnostic{}
	}
	rootDir := filepath.Dir(filepath.Clean(path))

	pipeline, err := model.ParseBytes([]byte(text))
	if err != nil {
		return []Diagnostic{{
			Range:    Range{Start: Position{}, End: Position{}},
			Severity: 1,
			Source:   "glint",
			Message:  "YAML parse error: " + err.Error(),
		}}
	}
	pipeline.SourceFile = path
	pipeline.SetJobOrigin(path)

	// Include resolution is best-effort: network failures produce warnings that
	// are intentionally discarded here. The linter operates on whatever was
	// successfully resolved.
	_, _ = resolver.ResolveIncludes(pipeline, s.cfg, rootDir)
	_, _ = resolver.Resolve(pipeline)

	findings := linter.Lint(pipeline, nil)

	diags := make([]Diagnostic, 0, len(findings))
	for _, f := range findings {
		// Skip findings from included files — their line numbers reference
		// a different document URI that the server has not opened.
		if f.File != path && f.File != "" {
			continue
		}
		line := 0
		if f.Line > 0 {
			line = f.Line - 1 // glint uses 1-based lines; LSP uses 0-based
		}
		sev := 1 // DiagnosticSeverity: Error
		if f.Severity == linter.Warning {
			sev = 2 // DiagnosticSeverity: Warning
		}
		msg := f.Message
		if f.Job != "" {
			msg = fmt.Sprintf("job %q: %s", f.Job, f.Message)
		}
		diags = append(diags, Diagnostic{
			Range: Range{
				Start: Position{Line: line},
				End:   Position{Line: line},
			},
			Severity: sev,
			Code:     f.Rule,
			Source:   "glint",
			Message:  msg,
		})
	}
	return diags
}

// uriToPath converts a file:// URI to a local filesystem path.
// Returns an empty string for non-file URIs or on parse error.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	return filepath.FromSlash(u.Path)
}
