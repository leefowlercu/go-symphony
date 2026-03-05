package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/leefowlercu/go-symphony/internal/codex/tools"
	"github.com/leefowlercu/go-symphony/internal/config"
	"github.com/leefowlercu/go-symphony/internal/domain"
)

type Client struct {
	logger      *slog.Logger
	settings    config.CodexConfig
	toolManager *tools.DynamicToolManager
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdoutLines chan string
	stdoutErr   chan error
	closed      chan struct{}
	nextID      atomic.Int64
	threadID    string
	workspace   string
	issue       domain.Issue
}

type Factory struct {
	Logger      *slog.Logger
	Settings    config.CodexConfig
	ToolManager *tools.DynamicToolManager
}

func (f *Factory) New(workspace string, issue domain.Issue) *Client {
	c := &Client{
		logger:      f.Logger,
		settings:    f.Settings,
		toolManager: f.ToolManager,
		stdoutLines: make(chan string, 256),
		stdoutErr:   make(chan error, 1),
		closed:      make(chan struct{}),
		workspace:   workspace,
		issue:       issue,
	}
	c.nextID.Store(10)
	return c
}

func (c *Client) Start(ctx context.Context) error {
	if err := c.launch(ctx); err != nil {
		return err
	}
	if err := c.sendRequest(ctx, 1, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "symphony", "version": "1.0.0"},
		"capabilities": map[string]any{"experimentalApi": true},
	}); err != nil {
		return err
	}
	if err := c.sendNotification("initialized", map[string]any{}); err != nil {
		return err
	}

	params := map[string]any{
		"approvalPolicy": c.settings.ApprovalPolicy,
		"sandbox":        c.settings.ThreadSandbox,
		"cwd":            c.workspace,
	}
	if c.toolManager != nil {
		params["dynamicTools"] = c.toolManager.ToolSpecs()
	}
	res, err := c.sendRequestResult(ctx, 2, "thread/start", params)
	if err != nil {
		return err
	}
	threadPayload, _ := res["thread"].(map[string]any)
	threadID, _ := threadPayload["id"].(string)
	if strings.TrimSpace(threadID) == "" {
		return fmt.Errorf("response_error: missing thread id")
	}
	c.threadID = threadID
	return nil
}

func (c *Client) RunTurn(ctx context.Context, prompt string, onEvent func(domain.AgentEvent)) (string, error) {
	id := c.nextID.Add(1)
	params := map[string]any{
		"threadId":       c.threadID,
		"input":          []map[string]any{{"type": "text", "text": prompt}},
		"cwd":            c.workspace,
		"title":          fmt.Sprintf("%s: %s", c.issue.Identifier, c.issue.Title),
		"approvalPolicy": c.settings.ApprovalPolicy,
		"sandboxPolicy":  c.settings.TurnSandboxPolicy,
	}
	res, err := c.sendRequestResult(ctx, id, "turn/start", params)
	if err != nil {
		return "", err
	}
	turnPayload, _ := res["turn"].(map[string]any)
	turnID, _ := turnPayload["id"].(string)
	if strings.TrimSpace(turnID) == "" {
		return "", fmt.Errorf("response_error: missing turn id")
	}
	sessionID := c.threadID + "-" + turnID
	onEvent(domain.AgentEvent{Event: "session_started", Timestamp: time.Now().UTC(), SessionID: sessionID})

	turnTimer := time.NewTimer(time.Duration(c.settings.TurnTimeoutMS) * time.Millisecond)
	defer turnTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return sessionID, ctx.Err()
		case <-turnTimer.C:
			return sessionID, fmt.Errorf("turn_timeout")
		case err := <-c.stdoutErr:
			if err != nil {
				return sessionID, fmt.Errorf("port_exit: %w", err)
			}
		case line := <-c.stdoutLines:
			msg, err := parseLine(line)
			if err != nil {
				onEvent(domain.AgentEvent{Event: "malformed", Timestamp: time.Now().UTC(), SessionID: sessionID, Message: truncate(line, 400)})
				continue
			}
			if msg.Method == "turn/completed" {
				onEvent(domain.AgentEvent{Event: "turn_completed", Timestamp: time.Now().UTC(), SessionID: sessionID, Payload: msg.Params})
				return sessionID, nil
			}
			if msg.Method == "turn/failed" {
				onEvent(domain.AgentEvent{Event: "turn_failed", Timestamp: time.Now().UTC(), SessionID: sessionID, Payload: msg.Params})
				return sessionID, fmt.Errorf("turn_failed")
			}
			if msg.Method == "turn/cancelled" {
				onEvent(domain.AgentEvent{Event: "turn_cancelled", Timestamp: time.Now().UTC(), SessionID: sessionID, Payload: msg.Params})
				return sessionID, fmt.Errorf("turn_cancelled")
			}
			if err := c.maybeHandleInteractive(ctx, msg, sessionID, onEvent); err != nil {
				return sessionID, err
			}
			extractUsageAndRateLimits(msg, sessionID, onEvent)
			onEvent(domain.AgentEvent{Event: "notification", Timestamp: time.Now().UTC(), SessionID: sessionID, Message: msg.Method, Payload: msg.Params})
		}
	}
}

func (c *Client) Stop() {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
}

func (c *Client) launch(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "bash", "-lc", c.settings.Command)
	cmd.Dir = c.workspace
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("codex_not_found: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("port_exit: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("port_exit: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("codex_not_found: %w", err)
	}
	c.cmd = cmd
	c.stdin = stdin
	go c.readStdout(stdout)
	go c.readStderr(stderr)
	go func() {
		_ = cmd.Wait()
		c.stdoutErr <- errors.New("process exited")
	}()
	return nil
}

func (c *Client) readStdout(r io.Reader) {
	reader := bufio.NewReaderSize(r, 10*1024*1024)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			select {
			case c.stdoutLines <- strings.TrimRight(line, "\r\n"):
			case <-c.closed:
				return
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			select {
			case c.stdoutErr <- err:
			default:
			}
			return
		}
	}
}

func (c *Client) readStderr(r io.Reader) {
	s := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	s.Buffer(buf, 10*1024*1024)
	for s.Scan() {
		c.logger.Debug("codex stderr", "line", truncate(s.Text(), 500))
	}
}

func (c *Client) sendRequest(ctx context.Context, id int64, method string, params map[string]any) error {
	_, err := c.sendRequestResult(ctx, id, method, params)
	return err
}

func (c *Client) sendRequestResult(ctx context.Context, id int64, method string, params map[string]any) (map[string]any, error) {
	payload := map[string]any{"id": id, "method": method, "params": params}
	if err := c.writeJSON(payload); err != nil {
		return nil, err
	}
	timer := time.NewTimer(time.Duration(c.settings.ReadTimeoutMS) * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, fmt.Errorf("response_timeout")
		case err := <-c.stdoutErr:
			return nil, fmt.Errorf("port_exit: %w", err)
		case line := <-c.stdoutLines:
			msg, err := parseLine(line)
			if err != nil {
				continue
			}
			if intify(msg.ID) != id {
				// Startup can emit notifications; ignore here.
				continue
			}
			if msg.Error != nil {
				return nil, fmt.Errorf("response_error: %v", msg.Error)
			}
			return msg.Result, nil
		}
	}
}

func (c *Client) sendNotification(method string, params map[string]any) error {
	return c.writeJSON(map[string]any{"method": method, "params": params})
}

func (c *Client) writeJSON(payload map[string]any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = c.stdin.Write(b)
	if err != nil {
		return fmt.Errorf("port_exit: %w", err)
	}
	return nil
}

func (c *Client) maybeHandleInteractive(ctx context.Context, msg message, sessionID string, onEvent func(domain.AgentEvent)) error {
	id := intify(msg.ID)
	if id == 0 {
		return nil
	}
	method := msg.Method
	if method == "item/tool/requestUserInput" || strings.Contains(strings.ToLower(method), "requestuserinput") {
		onEvent(domain.AgentEvent{Event: "turn_input_required", Timestamp: time.Now().UTC(), SessionID: sessionID, Payload: msg.Params})
		_ = c.writeJSON(map[string]any{"id": id, "result": map[string]any{"approved": false}})
		return fmt.Errorf("turn_input_required")
	}
	if strings.Contains(method, "approval") || method == "exec_command/request_approval" || method == "file_change/request_approval" {
		resp := map[string]any{"id": id, "result": map[string]any{"approved": true}}
		_ = c.writeJSON(resp)
		onEvent(domain.AgentEvent{Event: "approval_auto_approved", Timestamp: time.Now().UTC(), SessionID: sessionID, Payload: msg.Params})
		return nil
	}
	if method == "item/tool/call" {
		toolName, arguments := toolCallInfo(msg.Params)
		resp := map[string]any{"id": id, "result": map[string]any{"success": false, "error": "unsupported_tool_call"}}
		if c.toolManager != nil {
			resp["result"] = c.toolManager.Execute(ctx, toolName, arguments)
		}
		_ = c.writeJSON(resp)
		if toolName == "" {
			onEvent(domain.AgentEvent{Event: "unsupported_tool_call", Timestamp: time.Now().UTC(), SessionID: sessionID, Payload: msg.Params})
		}
		return nil
	}
	return nil
}

func extractUsageAndRateLimits(msg message, sessionID string, onEvent func(domain.AgentEvent)) {
	if msg.Params == nil {
		return
	}
	usage := findMapRecursive(msg.Params, "total_token_usage")
	if usage == nil {
		usage = findMapRecursive(msg.Params, "usage")
	}
	rateLimits := findMapRecursive(msg.Params, "rate_limits")
	if usage == nil && rateLimits == nil {
		return
	}
	onEvent(domain.AgentEvent{Event: "token_usage", Timestamp: time.Now().UTC(), SessionID: sessionID, Usage: usage, RateLimits: rateLimits, Payload: msg.Params})
}

func toolCallInfo(params map[string]any) (string, any) {
	name, _ := params["name"].(string)
	if name == "" {
		name, _ = params["tool"].(string)
	}
	return name, params["arguments"]
}

func findMapRecursive(root map[string]any, key string) map[string]any {
	if root == nil {
		return nil
	}
	if v, ok := root[key].(map[string]any); ok {
		return v
	}
	for _, v := range root {
		switch t := v.(type) {
		case map[string]any:
			if m := findMapRecursive(t, key); m != nil {
				return m
			}
		}
	}
	return nil
}

func intify(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "...<truncated>"
}
