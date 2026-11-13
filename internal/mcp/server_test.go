package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func roundTrip(t *testing.T, s *Server, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var msgs []map[string]any
	for _, l := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("output line is not JSON: %q", l)
		}
		if m["jsonrpc"] != "2.0" {
			t.Fatalf("missing jsonrpc 2.0: %q", l)
		}
		msgs = append(msgs, m)
	}
	return msgs
}

func echo() Tool {
	return Tool{Name: "echo", Description: "d", Schema: map[string]any{"type": "object"}, ReadOnly: true,
		Call: func(a json.RawMessage) (string, error) { return string(a), nil }}
}

func call(name, args string) string {
	return `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
}

func TestServe_Initialize(t *testing.T) {
	s := &Server{Name: "n", Version: "1", Instructions: "hi"}
	m := roundTrip(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	r := m[0]["result"].(map[string]any)
	if r["protocolVersion"] != "2025-06-18" || r["instructions"] != "hi" ||
		r["serverInfo"].(map[string]any)["name"] != "n" {
		t.Fatalf("bad result: %v", r)
	}
	if _, ok := r["capabilities"].(map[string]any)["tools"]; !ok {
		t.Fatalf("no tools capability: %v", r)
	}
	m = roundTrip(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`)
	if v := m[0]["result"].(map[string]any)["protocolVersion"]; v != "2025-11-25" {
		t.Fatalf("version = %v", v)
	}
}

func TestServe_NotificationsGetNoAnswer(t *testing.T) {
	if m := roundTrip(t, &Server{}, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); len(m) != 0 {
		t.Fatalf("got %v", m)
	}
}

func TestServe_ListAndCall(t *testing.T) {
	s := &Server{Tools: []Tool{echo()}}
	m := roundTrip(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, call("echo", `{"a":1}`))
	tool := m[0]["result"].(map[string]any)["tools"].([]any)[0].(map[string]any)
	if tool["name"] != "echo" || tool["annotations"].(map[string]any)["readOnlyHint"] != true {
		t.Fatalf("bad tool: %v", tool)
	}
	if _, ok := tool["inputSchema"]; !ok {
		t.Fatal("no inputSchema")
	}
	text := m[1]["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"]
	if text != `{"a":1}` {
		t.Fatalf("text = %v", text)
	}
}

func TestServe_ToolErrorIsAResult(t *testing.T) {
	s := &Server{Tools: []Tool{{Name: "x", Call: func(json.RawMessage) (string, error) { return "", errors.New("boom") }}}}
	m := roundTrip(t, s, call("x", `{}`))[0]
	r := m["result"].(map[string]any)
	if _, ok := m["error"]; ok || r["isError"] != true ||
		r["content"].([]any)[0].(map[string]any)["text"] != "boom" {
		t.Fatalf("bad: %v", m)
	}
}

func TestServe_BeforeErrorIsAResult(t *testing.T) {
	ran := false
	s := &Server{
		Before: func() error { return errors.New("stale") },
		Tools:  []Tool{{Name: "x", Call: func(json.RawMessage) (string, error) { ran = true; return "", nil }}},
	}
	r := roundTrip(t, s, call("x", `{}`))[0]["result"].(map[string]any)
	if ran || r["isError"] != true || r["content"].([]any)[0].(map[string]any)["text"] != "stale" {
		t.Fatalf("bad: ran=%v %v", ran, r)
	}
}

func TestServe_Errors(t *testing.T) {
	m := roundTrip(t, &Server{},
		call("nope", `{}`),
		`{"jsonrpc":"2.0","id":3,"method":"resources/list"}`,
		`not json`,
		`{"jsonrpc":"2.0","id":4,"method":"ping"}`)
	code := func(i int) any { return m[i]["error"].(map[string]any)["code"] }
	if code(0) != float64(-32602) || code(1) != float64(-32601) || code(2) != float64(-32700) {
		t.Fatalf("codes: %v", m)
	}
	if m[0]["error"].(map[string]any)["message"] != "unknown tool: nope" {
		t.Fatalf("message: %v", m[0])
	}
	if id, ok := m[2]["id"]; !ok || id != nil {
		t.Fatalf("parse error id: %v", m[2])
	}
	if _, ok := m[3]["result"]; !ok {
		t.Fatalf("ping not answered: %v", m[3])
	}
}

func TestServe_LongLine(t *testing.T) {
	s := &Server{Tools: []Tool{echo()}}
	m := roundTrip(t, s, call("echo", `{"a":"`+strings.Repeat("x", 200<<10)+`"}`))
	if len(m) != 1 || m[0]["result"] == nil {
		t.Fatalf("not answered")
	}
}

func TestServe_EOF(t *testing.T) {
	if err := (&Server{}).Serve(strings.NewReader(""), io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestServe_APanickingToolDoesNotKillTheServer(t *testing.T) {
	boom := Tool{Name: "boom", Call: func(json.RawMessage) (string, error) { panic("kaput") }}
	m := roundTrip(t, &Server{Tools: []Tool{boom}}, call("boom", `{}`), `{"jsonrpc":"2.0","id":3,"method":"ping"}`)
	res := m[0]["result"].(map[string]any)
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	if res["isError"] != true || !strings.Contains(text, "kaput") {
		t.Fatalf("result = %v", res)
	}
	if len(m) != 2 || m[1]["id"] != float64(3) {
		t.Fatalf("ping not answered: %v", m)
	}
}

func TestServe_APanickingBeforeDoesNotKillTheServer(t *testing.T) {
	s := &Server{
		Before: func() error { panic("reload") },
		Tools:  []Tool{{Name: "x", Call: func(json.RawMessage) (string, error) { return "", nil }}},
	}
	m := roundTrip(t, s, call("x", `{}`), `{"jsonrpc":"2.0","id":3,"method":"ping"}`)
	res := m[0]["result"].(map[string]any)
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	if res["isError"] != true || !strings.Contains(text, "reload") || len(m) != 2 {
		t.Fatalf("result = %v, replies = %d", res, len(m))
	}
}

func TestServe_NilSchemaIsAnEmptyObjectSchema(t *testing.T) {
	m := roundTrip(t, &Server{Tools: []Tool{{Name: "x"}}}, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	tool := m[0]["result"].(map[string]any)["tools"].([]any)[0].(map[string]any)
	if s, _ := tool["inputSchema"].(map[string]any); s["type"] != "object" {
		t.Fatalf("inputSchema = %v", tool["inputSchema"])
	}
}
