package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"oralarchive/internal/application"
	"oralarchive/internal/domain"
	"oralarchive/internal/httpui"
	"oralarchive/internal/repository"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19091", "监听地址")
	selfcheck := flag.Bool("selfcheck", false, "运行完整自检")
	dbPath := flag.String("db", "oralarchive.db", "SQLite 文件路径")
	flag.Parse()
	if env := os.Getenv("PORT"); env != "" && !flagProvided("addr") {
		if _, err := strconv.Atoi(env); err == nil {
			*addr = "127.0.0.1:" + env
		}
	}
	if err := validateAddr(*addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	actualDB := *dbPath
	var cleanup func() = func() {}
	if *selfcheck {
		f, err := os.CreateTemp("", "oralarchive-selfcheck-*.db")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		actualDB = f.Name()
		f.Close()
		cleanup = func() { os.Remove(actualDB) }
	}
	store, err := repository.Open(actualDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	defer cleanup()
	app := application.New(store)
	server := &http.Server{Addr: *addr, Handler: httpui.New(app).Handler(), ReadHeaderTimeout: 5 * time.Second}
	if *selfcheck {
		if err := runSelfcheck(server, *addr); err != nil {
			fmt.Fprintln(os.Stderr, "自检失败:", err)
			os.Exit(1)
		}
		return
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("口述史工作台监听", listener.Addr().String())
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			fmt.Fprintln(os.Stderr, "关闭服务失败:", err)
			os.Exit(1)
		}
		if err := <-serveErrors; err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case err := <-serveErrors:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func flagProvided(name string) bool {
	for _, arg := range os.Args[1:] {
		if arg == "-"+name || strings.HasPrefix(arg, "-"+name+"=") {
			return true
		}
	}
	return false
}
func validateAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("监听地址无效: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("监听地址必须为回环地址")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1024 || p > 65535 {
		return fmt.Errorf("监听端口必须在 1024-65535")
	}
	return nil
}

func runSelfcheck(server *http.Server, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go server.Serve(listener)
	defer server.Shutdown(context.Background())
	base := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 5 * time.Second}
	request := func(method, path string, body any) (map[string]any, error) {
		var reader io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			reader = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, base+path, reader)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var out map[string]any
		json.Unmarshal(raw, &out)
		if resp.StatusCode >= 300 {
			return out, fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, string(raw))
		}
		return out, nil
	}
	if _, err := request("GET", "/healthz", nil); err != nil {
		return err
	}
	hash := domain.Digest("selfcheck")
	create := map[string]any{"request_id": "sc-create", "dossier_id": "sc-001", "subject_code": "SUB-OLD", "audio_ref": "vault://sc.wav", "audio_sha256": hash, "interviewed_at": "2026-08-27", "allowed_uses": []string{"research"}, "embargo_until": "2030-01-01", "consent_evidence_digest": hash, "editor_id": "editor-1"}
	out, err := request("POST", "/api/dossiers", create)
	if err != nil {
		return err
	}
	id := out["dossier_id"].(string)
	rev := int64(out["revision"].(float64))
	revise := map[string]any{"request_id": "sc-revise", "expected_revision": rev, "actor_id": "editor-1", "subject_code": "SUB-1", "audio_ref": "vault://sc.wav", "audio_sha256": hash, "interviewed_at": "2026-08-27", "allowed_uses": []string{" research ", "research"}, "embargo_until": "2030-01-01", "consent_evidence_digest": hash}
	out, err = request("PATCH", "/api/dossiers/"+id+"/consent", revise)
	if err != nil {
		return err
	}
	rev = int64(out["revision"].(float64))
	action := func(path, actor string) (map[string]any, error) {
		nonlocal := map[string]any{"request_id": "sc-" + strings.ReplaceAll(path, "/", "-"), "expected_revision": rev, "actor_id": actor}
		o, e := request("POST", "/api/dossiers/"+id+path, nonlocal)
		if e == nil {
			rev = int64(o["revision"].(float64))
		}
		return o, e
	}
	if _, err = action("/consent/lock", "editor-1"); err != nil {
		return err
	}
	operations := []any{map[string]any{"type": "add", "segment_id": "s1", "segment": map[string]any{"segment_id": "s1", "start_ms": 0, "end_ms": 1000, "speaker_code": "INTERVIEWER", "text": "联系电话 13800138000"}}}
	if preflight, preflightErr := request("POST", "/api/dossiers/"+id+"/transcript/precheck", map[string]any{"operations": operations}); preflightErr != nil || preflight["valid"] != true {
		return fmt.Errorf("时间轴预检失败: %v", preflightErr)
	}
	trans := map[string]any{"request_id": "sc-transcript", "expected_revision": rev, "actor_id": "editor-1", "operations": operations}
	o, err := request("POST", "/api/dossiers/"+id+"/transcript/operations", trans)
	if err != nil {
		return err
	}
	rev = int64(o["revision"].(float64))
	if _, err = action("/transcript/freeze", "editor-1"); err != nil {
		return err
	}
	if _, err = action("/checks", "editor-1"); err != nil {
		return err
	}
	detail, err := request("GET", "/api/dossiers/"+id, nil)
	if err != nil {
		return err
	}
	issues := detail["dossier"].(map[string]any)["issues"].([]any)
	if len(issues) == 0 {
		return fmt.Errorf("检查未发现问题")
	}
	issue := issues[0].(map[string]any)
	segmentHash := detail["dossier"].(map[string]any)["segments"].([]any)[0].(map[string]any)["text_sha256"]
	resolve := map[string]any{"request_id": "sc-resolve", "expected_revision": rev, "items": []any{map[string]any{"issue_id": issue["issue_id"], "start_offset": int(issue["start_offset"].(float64)), "end_offset": int(issue["end_offset"].(float64)), "reason": "手机号遮蔽", "replacement_text": "[已遮蔽]", "actor_id": "editor-1", "segment_text_sha256": segmentHash}}}
	o, err = request("POST", "/api/dossiers/"+id+"/issues/batch-resolve", resolve)
	if err != nil {
		return err
	}
	rev = int64(o["revision"].(float64))
	detail, err = request("GET", "/api/dossiers/"+id, nil)
	if err != nil {
		return err
	}
	candidate := detail["candidate_sha256"].(string)
	confirm := map[string]any{"request_id": "sc-confirm", "expected_revision": rev, "decision": "approved", "confirmed_by": "subject-1", "evidence_digest": hash, "candidate_sha256": candidate, "allowed_exceptions": []string{}, "note": "已确认"}
	o, err = request("POST", "/api/dossiers/"+id+"/confirmations", confirm)
	if err != nil {
		return err
	}
	rev = int64(o["revision"].(float64))
	detail, err = request("GET", "/api/dossiers/"+id, nil)
	if err != nil {
		return err
	}
	checklist := detail["review_checklist"].([]any)
	for _, raw := range checklist {
		raw.(map[string]any)["conclusion"] = "passed"
	}
	review := map[string]any{"request_id": "sc-review", "expected_revision": rev, "decision": "approved", "reviewer_id": "reviewer-1", "reason": "通过", "checklist": checklist}
	o, err = request("POST", "/api/dossiers/"+id+"/reviews", review)
	if err != nil {
		return err
	}
	if o["status"].(string) != "sealed" {
		return fmt.Errorf("未进入封存状态")
	}
	final, err := request("GET", "/api/dossiers/"+id, nil)
	if err != nil {
		return err
	}
	if final["package_valid"] != true {
		return fmt.Errorf("发布包校验失败")
	}
	proof, err := request("GET", "/api/dossiers/"+id+"/release/verification", nil)
	if err != nil || proof["valid"] != true {
		return fmt.Errorf("发布证明校验失败: %v", err)
	}
	if final["dossier"].(map[string]any)["segments"].([]any)[0].(map[string]any)["text"] != "" {
		return fmt.Errorf("终态泄露原文")
	}
	queue, err := request("GET", "/api/dossiers?status=sealed&page_size=10", nil)
	if err != nil || len(queue["dossiers"].([]any)) != 1 {
		return fmt.Errorf("封存队列查询失败: %v", err)
	}
	fmt.Println("自检通过:", id)
	return nil
}
