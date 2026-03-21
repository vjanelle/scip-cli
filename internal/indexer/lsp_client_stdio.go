package indexer

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type stdioLSPClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	reader  *bufio.Reader
	pending map[int64]chan rpcResponse
	mu      sync.Mutex
	nextID  int64
	done    chan struct{}
	err     error
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func startLSPClient(ctx context.Context, req IndexRequest) (lspClient, []string, error) {
	cmd := exec.CommandContext(ctx, req.LSPCommand, req.LSPArgs...)
	cmd.Dir = req.Root
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env, req.LSPEnv...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	client := &stdioLSPClient{
		cmd:     cmd,
		stdin:   stdin,
		reader:  bufio.NewReader(stdout),
		pending: map[int64]chan rpcResponse{},
		done:    make(chan struct{}),
	}
	// A dedicated read loop lets request handlers remain synchronous while the
	// stdio transport continues to process LSP responses in the background.
	go client.readLoop()
	go io.Copy(io.Discard, stderr)
	return client, append([]string{req.LSPCommand}, req.LSPArgs...), nil
}

func (c *stdioLSPClient) Initialize(ctx context.Context, root string, initOptions map[string]any) error {
	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   root,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"documentSymbol": map[string]any{
					"hierarchicalDocumentSymbolSupport": true,
				},
			},
		},
		"workspaceFolders": []map[string]string{{"uri": root, "name": filepath.Base(strings.TrimPrefix(root, "file://"))}},
	}
	if len(initOptions) > 0 {
		params["initializationOptions"] = initOptions
	}
	if err := c.call(ctx, "initialize", params, nil); err != nil {
		return err
	}
	return c.notify("initialized", map[string]any{})
}

func (c *stdioLSPClient) OpenDocument(_ context.Context, uri, language, text string) error {
	return c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": language,
			"version":    1,
			"text":       text,
		},
	})
}

func (c *stdioLSPClient) DocumentSymbols(ctx context.Context, path, uri, source string) ([]Symbol, []lspSymbolRef, error) {
	var payload json.RawMessage
	if err := c.call(ctx, "textDocument/documentSymbol", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	}, &payload); err != nil {
		return nil, nil, err
	}
	return decodeDocumentSymbols(path, source, payload)
}

func (c *stdioLSPClient) References(ctx context.Context, uri string, position lspPosition) ([]lspLocation, error) {
	var locations []lspLocation
	if err := c.call(ctx, "textDocument/references", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     position,
		"context":      map[string]any{"includeDeclaration": true},
	}, &locations); err != nil {
		return nil, err
	}
	return locations, nil
}

func (c *stdioLSPClient) Close(ctx context.Context) error {
	_ = c.call(ctx, "shutdown", map[string]any{}, nil)
	_ = c.notify("exit", map[string]any{})
	<-c.done
	return c.err
}

func (c *stdioLSPClient) call(ctx context.Context, method string, params any, result any) error {
	id := atomic.AddInt64(&c.nextID, 1)
	responseCh := make(chan rpcResponse, 1)

	c.mu.Lock()
	c.pending[id] = responseCh
	c.mu.Unlock()

	if err := c.send(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case response := <-responseCh:
		if response.Error != nil {
			return errors.New(response.Error.Message)
		}
		if result != nil && len(response.Result) > 0 {
			return json.Unmarshal(response.Result, result)
		}
		return nil
	}
}

func (c *stdioLSPClient) notify(method string, params any) error {
	return c.send(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
}

func (c *stdioLSPClient) send(payload rpcRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	_, err = io.WriteString(c.stdin, message)
	return err
}

func (c *stdioLSPClient) readLoop() {
	defer close(c.done)
	for {
		body, err := readPacket(c.reader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.err = err
			}
			return
		}

		var response rpcResponse
		if err := json.Unmarshal(body, &response); err != nil {
			continue
		}
		if response.ID == 0 {
			continue
		}

		c.mu.Lock()
		responseCh := c.pending[response.ID]
		delete(c.pending, response.ID)
		c.mu.Unlock()
		if responseCh != nil {
			responseCh <- response
		}
	}
}

func readPacket(reader *bufio.Reader) ([]byte, error) {
	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if value, ok := strings.CutPrefix(strings.ToLower(line), "content-length: "); ok {
			contentLength, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
		}
	}
	if contentLength <= 0 {
		return nil, errors.New("missing content-length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}
