package gptscript

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	serverProcess       *exec.Cmd
	serverProcessCancel context.CancelFunc
	clientCount         int
	lock                sync.Mutex
)

const relativeToBinaryPath = "<me>"

type Client interface {
	Run(context.Context, string, Options) (*Run, error)
	Evaluate(context.Context, Options, ...fmt.Stringer) (*Run, error)
	Parse(ctx context.Context, fileName string) ([]Node, error)
	ParseTool(ctx context.Context, toolDef string) ([]Node, error)
	Version(ctx context.Context) (string, error)
	Fmt(ctx context.Context, nodes []Node) (string, error)
	ListTools(ctx context.Context) (string, error)
	ListModels(ctx context.Context) ([]string, error)
	Close()
}

type client struct {
	gptscriptURL string
}

func NewClient() (Client, error) {
	lock.Lock()
	defer lock.Unlock()
	clientCount++

	serverURL := os.Getenv("GPTSCRIPT_URL")
	if serverURL == "" {
		serverURL = "127.0.0.1:9090"
	}

	if serverProcessCancel == nil && os.Getenv("GPTSCRIPT_DISABLE_SERVER") != "true" {
		var ctx context.Context
		ctx, serverProcessCancel = context.WithCancel(context.Background())

		command := getCommand()
		serverProcess = exec.CommandContext(ctx, command, "--listen-address", serverURL, "clicky")
		if err := serverProcess.Start(); err != nil {
			serverProcessCancel()
			return nil, fmt.Errorf("failed to start server: %w", err)
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := waitForServerReady(timeoutCtx, serverURL); err != nil {
			serverProcessCancel()
			_ = serverProcess.Wait()
			return nil, fmt.Errorf("failed to wait for gptscript to be ready: %w", err)
		}
	}
	return &client{gptscriptURL: "http://" + serverURL}, nil
}

func waitForServerReady(ctx context.Context, serverURL string) error {
	for {
		resp, err := http.Get("http://" + serverURL + "/healthz")
		if err != nil {
			slog.DebugContext(ctx, "waiting for server to become ready")
		} else {
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (c *client) Close() {
	lock.Lock()
	defer lock.Unlock()
	clientCount--

	if clientCount == 0 && serverProcessCancel != nil {
		serverProcessCancel()
		_ = serverProcess.Wait()
	}
}

func (c *client) Evaluate(ctx context.Context, opts Options, tools ...fmt.Stringer) (*Run, error) {
	return (&Run{
		url:         c.gptscriptURL,
		requestPath: "evaluate",
		state:       Creating,
		opts:        opts,
		content:     concatTools(tools),
		chatState:   opts.ChatState,
	}).NextChat(ctx, opts.Input)
}

func (c *client) Run(ctx context.Context, toolPath string, opts Options) (*Run, error) {
	return (&Run{
		url:         c.gptscriptURL,
		requestPath: "run",
		state:       Creating,
		opts:        opts,
		toolPath:    toolPath,
		chatState:   opts.ChatState,
	}).NextChat(ctx, opts.Input)
}

// Parse will parse the given file into an array of Nodes.
func (c *client) Parse(ctx context.Context, fileName string) ([]Node, error) {
	out, err := c.runBasicCommand(ctx, "parse", fileName, "")
	if err != nil {
		return nil, err
	}

	var doc Document
	if err = json.Unmarshal([]byte(out), &doc); err != nil {
		return nil, err
	}

	return doc.Nodes, nil
}

// ParseTool will parse the given string into a tool.
func (c *client) ParseTool(ctx context.Context, toolDef string) ([]Node, error) {
	out, err := c.runBasicCommand(ctx, "parse", "", toolDef)
	if err != nil {
		return nil, err
	}

	var doc Document
	if err = json.Unmarshal([]byte(out), &doc); err != nil {
		return nil, err
	}

	return doc.Nodes, nil
}

// Fmt will format the given nodes into a string.
func (c *client) Fmt(ctx context.Context, nodes []Node) (string, error) {
	b, err := json.Marshal(Document{Nodes: nodes})
	if err != nil {
		return "", fmt.Errorf("failed to marshal nodes: %w", err)
	}

	run := &runSubCommand{
		Run: Run{
			url:         c.gptscriptURL,
			requestPath: "fmt",
			state:       Creating,
			toolPath:    "",
			content:     string(b),
		},
	}

	if err = run.request(ctx, Document{Nodes: nodes}); err != nil {
		return "", err
	}

	out, err := run.Text()
	if err != nil {
		return "", err
	}
	if run.err != nil {
		return run.ErrorOutput(), run.err
	}

	return out, nil
}

// Version will return the output of `gptscript --version`
func (c *client) Version(ctx context.Context) (string, error) {
	out, err := c.runBasicCommand(ctx, "version", "", "")
	if err != nil {
		return "", err
	}

	return out, nil
}

// ListTools will list all the available tools.
func (c *client) ListTools(ctx context.Context) (string, error) {
	out, err := c.runBasicCommand(ctx, "list-tools", "", "")
	if err != nil {
		return "", err
	}

	return out, nil
}

// ListModels will list all the available models.
func (c *client) ListModels(ctx context.Context) ([]string, error) {
	out, err := c.runBasicCommand(ctx, "list-models", "", "")
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimSpace(out), "\n"), nil
}

func (c *client) runBasicCommand(ctx context.Context, requestPath, toolPath, content string) (string, error) {
	run := &runSubCommand{
		Run: Run{
			url:         c.gptscriptURL,
			requestPath: requestPath,
			state:       Creating,
			toolPath:    toolPath,
			content:     content,
		},
	}

	var m any
	if content != "" || toolPath != "" {
		m = map[string]any{"content": content, "file": toolPath}
	}

	if err := run.request(ctx, m); err != nil {
		return "", err
	}

	out, err := run.Text()
	if err != nil {
		return "", err
	}
	if run.err != nil {
		return run.ErrorOutput(), run.err
	}

	return out, nil
}

func getCommand() string {
	if gptScriptBin := os.Getenv("GPTSCRIPT_BIN"); gptScriptBin != "" {
		if len(os.Args) == 0 {
			return gptScriptBin
		}
		return determineProperCommand(filepath.Dir(os.Args[0]), gptScriptBin)
	}

	return "gptscript"
}

// determineProperCommand is for testing purposes. Users should use getCommand instead.
func determineProperCommand(dir, bin string) string {
	if !strings.HasPrefix(bin, relativeToBinaryPath) {
		return bin
	}

	bin = filepath.Join(dir, strings.TrimPrefix(bin, relativeToBinaryPath))
	if !filepath.IsAbs(bin) {
		bin = "." + string(os.PathSeparator) + bin
	}

	slog.Debug("Using gptscript binary: " + bin)
	return bin
}
