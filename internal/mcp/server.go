// Package mcp is a small tools-only Model Context Protocol server: JSON-RPC
// 2.0, one message per line, over a reader and a writer.
package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Tool is one thing a client may call.
type Tool struct {
	Name        string
	Description string
	// Schema is the JSON Schema of the arguments, an object schema.
	Schema map[string]any
	// ReadOnly marks a tool that changes nothing.
	ReadOnly bool
	// Call runs it. The returned text is the result; an error becomes an
	// isError result, not a protocol error.
	Call func(arguments json.RawMessage) (string, error)
}

type Server struct {
	Name, Version, Instructions string
	Tools                       []Tool
	// Before runs ahead of every tools/call; its error becomes an isError
	// result. The command uses it to reload the configuration.
	Before func() error
}

// Newest first; initialize falls back to the first.
var protocolVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Serve answers requests from in on out, one JSON message per line, until
// in ends. It returns nil on EOF.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	for {
		line, err := r.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			if werr := s.handle(line, out); werr != nil {
				return werr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (s *Server) handle(line []byte, out io.Writer) error {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return reply(out, json.RawMessage("null"), nil, &rpcError{-32700, "parse error"})
	}
	if len(req.ID) == 0 { // a notification is never answered
		return nil
	}
	result, rerr := s.dispatch(req)
	return reply(out, req.ID, result, rerr)
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func reply(out io.Writer, id json.RawMessage, result any, rerr *rpcError) error {
	msg := map[string]any{"jsonrpc": "2.0", "id": id}
	if rerr != nil {
		msg["error"] = rerr
	} else {
		msg["result"] = result
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

func (s *Server) dispatch(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		v := protocolVersions[0]
		if slices.Contains(protocolVersions, p.ProtocolVersion) {
			v = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
			"instructions":    s.Instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.Tools))
		for _, t := range s.Tools {
			schema := t.Schema
			if schema == nil { // a client rejects a null schema
				schema = map[string]any{"type": "object"}
			}
			tools = append(tools, map[string]any{
				"name": t.Name, "description": t.Description, "inputSchema": schema,
				"annotations": map[string]any{"readOnlyHint": t.ReadOnly},
			})
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		return s.call(req.Params)
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func (s *Server) call(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{-32602, "invalid params"}
	}
	i := slices.IndexFunc(s.Tools, func(t Tool) bool { return t.Name == p.Name })
	if i < 0 {
		return nil, &rpcError{-32602, fmt.Sprintf("unknown tool: %s", p.Name)}
	}
	text, err := callTool(s.Before, s.Tools[i], p.Arguments)
	res := map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
	if err != nil {
		res["content"] = []any{map[string]any{"type": "text", "text": err.Error()}}
		res["isError"] = true
	}
	return res, nil
}

// callTool runs before and then a tool; a panic in either must not take the
// server down.
func callTool(before func() error, t Tool, arguments json.RawMessage) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text, err = "", fmt.Errorf("tool %s panicked: %v", t.Name, r)
		}
	}()
	if before != nil {
		if err := before(); err != nil {
			return "", err
		}
	}
	return t.Call(arguments)
}
