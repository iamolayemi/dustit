package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/doctor"
	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/history"
	"github.com/iamolayemi/dustit/internal/targets"
	"github.com/iamolayemi/dustit/internal/ui"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run as an MCP server so AI agents can inspect and clean safely",
		Long: `Speaks the Model Context Protocol over stdin/stdout, so AI coding agents can ask DustIt
what's using space, check the Mac's health and clean up through DustIt's safety rules
instead of improvising rm -rf.

Safety: clean-ups are dry runs unless the agent passes dry_run=false; "review" categories
are refused unless it passes allow_review=true; every removal goes through the same guard
as the CLI and is logged, so "dustit undo" can put trashed items back.

Add it to Claude Code:   claude mcp add dustit -- dustit mcp
Or to any MCP client:    {"command": "dustit", "args": ["mcp"]}`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return serveMCP(os.Stdin, os.Stdout) },
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
	run         func(args map[string]any) (any, error)
}

func obj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func readOnly() map[string]any { return map[string]any{"readOnlyHint": true} }

func mcpTools() []mcpTool {
	return []mcpTool{
		{Name: "scan", Description: "Measure every cleanup category (read-only). Returns sizes, item counts and whether each category is safe or needs review.",
			InputSchema: obj(map[string]any{"include_items": map[string]any{"type": "boolean", "description": "include every item path (can be long)"}}),
			Annotations: readOnly(),
			run: func(a map[string]any) (any, error) {
				res := targets.ScanAll(targets.All())
				return scanJSON(res, nil, fsx.FreeSpace(), boolArg(a, "include_items", false)), nil
			}},
		{Name: "storage", Description: "Where disk space went, in plain groups (Applications, Downloads, Photos, app data, developer files, macOS…) with how to free each.",
			InputSchema: obj(map[string]any{}), Annotations: readOnly(),
			run: func(map[string]any) (any, error) { return finder.Storage(), nil }},
		{Name: "doctor", Description: "Diagnose why the Mac is slow: CPU hogs, memory pressure, disk, battery, Spotlight, browsers, startup items. Read-only.",
			InputSchema: obj(map[string]any{}), Annotations: readOnly(),
			run: func(map[string]any) (any, error) { return doctorJSON(doctor.Run(ui.Human)), nil }},
		{Name: "list_items", Description: "List what one category would remove (paths, sizes, notes). Read-only.",
			InputSchema: obj(map[string]any{"category": map[string]any{"type": "string", "description": "category id, e.g. npm-cache"},
				"limit": map[string]any{"type": "integer", "description": "max items (default 50)"}}, "category"),
			Annotations: readOnly(),
			run: func(a map[string]any) (any, error) {
				sel, err := targets.Select([]string{strArg(a, "category")}, false)
				if err != nil {
					return nil, err
				}
				r := targets.Scan(sel[0])
				limit := intArg(a, "limit", 50)
				var items []jsonItem
				for i, it := range r.Items {
					if i == limit {
						break
					}
					items = append(items, jsonItem{it.Path, it.Size, it.Note})
				}
				return map[string]any{"category": r.T.ID, "risk": r.T.Risk.String(), "size_bytes": r.Bytes, "item_count": len(r.Items), "items": items}, nil
			}},
		{Name: "clean", Description: "Clean categories through DustIt's safety guard. Dry run by default: pass dry_run=false to actually remove. Review categories need allow_review=true. Personal files go to the Trash and can be restored with the undo tool.",
			InputSchema: obj(map[string]any{
				"categories":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "category ids (see scan)"},
				"dry_run":      map[string]any{"type": "boolean", "description": "default true: only report what would be removed"},
				"allow_review": map[string]any{"type": "boolean", "description": "allow categories marked review (they may hold things the user wants)"},
			}, "categories"),
			Annotations: map[string]any{"destructiveHint": true},
			run:         mcpClean},
		{Name: "project", Description: "What a project folder uses: dependency/build folders, .git size, merged branches, Docker containers/images/volumes. Read-only.",
			InputSchema: obj(map[string]any{"path": map[string]any{"type": "string", "description": "project folder (absolute or ~/…)"}}, "path"),
			Annotations: readOnly(),
			run: func(a map[string]any) (any, error) {
				p := finder.InspectProject(fsx.Expand(strArg(a, "path")))
				var deps []jsonItem
				for _, it := range p.Artifacts {
					deps = append(deps, jsonItem{it.Path, it.Size, it.Note})
				}
				out := map[string]any{"root": p.Root, "dependencies": deps, "git_bytes": p.GitSize, "docker": p.Docker}
				if p.Branches != nil {
					out["merged_branches"] = p.Branches.Note
				}
				return out, nil
			}},
		{Name: "history", Description: "Recent clean-up runs: what was trashed or deleted and whether it can be undone.",
			InputSchema: obj(map[string]any{}), Annotations: readOnly(),
			run: func(map[string]any) (any, error) {
				recs, err := history.Load()
				if err != nil {
					return nil, err
				}
				runs := history.Runs(recs)
				if len(runs) > 20 {
					runs = runs[:20]
				}
				return runs, nil
			}},
		{Name: "undo", Description: "Put back what a run moved to the Trash (the latest undoable run if run_id is empty).",
			InputSchema: obj(map[string]any{"run_id": map[string]any{"type": "string"}}),
			run: func(a map[string]any) (any, error) {
				recs, err := history.Load()
				if err != nil {
					return nil, err
				}
				r, err := history.Find(history.Runs(recs), strArg(a, "run_id"))
				if err != nil {
					return nil, err
				}
				res := history.Undo(r)
				return map[string]any{"run": r.ID, "restored": res.Restored, "bytes": res.Bytes, "missing": res.Missing, "occupied": res.Occupied}, nil
			}},
	}
}

func mcpClean(a map[string]any) (any, error) {
	var ids []string
	if raw, ok := a["categories"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				ids = append(ids, s)
			}
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("pass at least one category id (see the scan tool)")
	}
	sel, err := targets.Select(ids, false)
	if err != nil {
		return nil, err
	}
	dry := boolArg(a, "dry_run", true)
	allowReview := boolArg(a, "allow_review", false)
	history.Begin("dustit mcp clean")
	type result struct {
		Category string   `json:"category"`
		Freed    int64    `json:"freed_bytes"`
		Skipped  int      `json:"skipped"`
		Log      []string `json:"log,omitempty"`
		Refused  string   `json:"refused,omitempty"`
	}
	var out []result
	var total int64
	for _, t := range sel {
		if t.Risk == targets.Review && !allowReview {
			out = append(out, result{Category: t.ID, Refused: "review category: ask the user, then pass allow_review=true"})
			continue
		}
		r := targets.Scan(t)
		var log []string
		freed, failed := targets.Clean(r, targets.CleanOptions{DryRun: dry, Log: func(l string) { log = append(log, l) }})
		total += freed
		out = append(out, result{Category: t.ID, Freed: freed, Skipped: failed, Log: log})
	}
	return map[string]any{"dry_run": dry, "total_bytes": total, "results": out, "free_bytes_now": fsx.FreeSpace()}, nil
}

func strArg(a map[string]any, k string) string {
	s, _ := a[k].(string)
	return s
}

func boolArg(a map[string]any, k string, def bool) bool {
	if b, ok := a[k].(bool); ok {
		return b
	}
	return def
}

func intArg(a map[string]any, k string, def int) int {
	if f, ok := a[k].(float64); ok && f > 0 {
		return int(f)
	}
	return def
}

// serveMCP handles newline-delimited JSON-RPC 2.0 messages until in closes.
func serveMCP(in io.Reader, out io.Writer) error {
	tools := mcpTools()
	byName := map[string]mcpTool{}
	for _, t := range tools {
		byName[t.Name] = t
	}
	var mu sync.Mutex
	send := func(v any) {
		b, _ := json.Marshal(v)
		mu.Lock()
		_, _ = out.Write(append(b, '\n'))
		mu.Unlock()
	}
	reply := func(id json.RawMessage, result any, rerr *rpcError) {
		msg := map[string]any{"jsonrpc": "2.0", "id": id}
		if rerr != nil {
			msg["error"] = rerr
		} else {
			msg["result"] = result
		}
		send(msg)
	}

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			reply(json.RawMessage("null"), nil, &rpcError{-32700, "parse error"})
			continue
		}
		if len(req.ID) == 0 { // notification (e.g. notifications/initialized): no reply
			continue
		}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2025-06-18"
			}
			reply(req.ID, map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "dustit", "version": version},
				"instructions": "DustIt cleans macOS disks safely. Use scan/storage/project to look, list_items to see paths, " +
					"and clean with dry_run=true first. Ask the user before dry_run=false, and before allow_review=true.",
			}, nil)
		case "ping":
			reply(req.ID, map[string]any{}, nil)
		case "tools/list":
			reply(req.ID, map[string]any{"tools": tools}, nil)
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil {
				reply(req.ID, nil, &rpcError{-32602, "invalid params"})
				continue
			}
			t, ok := byName[p.Name]
			if !ok {
				reply(req.ID, nil, &rpcError{-32602, "unknown tool " + p.Name})
				continue
			}
			if p.Arguments == nil {
				p.Arguments = map[string]any{}
			}
			res, err := t.run(p.Arguments)
			if err != nil {
				reply(req.ID, map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}, nil)
				continue
			}
			b, _ := json.MarshalIndent(res, "", "  ")
			reply(req.ID, map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, nil)
		default:
			reply(req.ID, nil, &rpcError{-32601, "method not found: " + req.Method})
		}
	}
	return sc.Err()
}
