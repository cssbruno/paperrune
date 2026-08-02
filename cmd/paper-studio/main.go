// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

// Command paper-studio serves the read-first Paper Studio workspace. Browser
// code never performs document layout: every visible page and overlay is an
// immutable artifact captured from the shared Paper display plan.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cssbruno/paperrune/document"
	"github.com/cssbruno/paperrune/internal/paperassets"
	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

var version = "devel"

const (
	studioSourceLimit        = 8 << 20
	studioJSONLimit          = 1 << 20
	studioAPITimeout         = 15 * time.Second
	studioChangePollInterval = 250 * time.Millisecond
	studioChangeKeepAlive    = 15 * time.Second
	studioScenarioCacheLimit = 32
	studioInspectionLimit    = 128
	studioPageRailLimit      = 10_000
	studioPageIssueLimit     = 16
	studioSessionTokenBytes  = 32
	studioSessionHeader      = "X-Paper-Studio-Token"
)

// Keep the raw WASM build artifact out of the native server binary. The
// deterministic Brotli or gzip representation is served directly to browsers;
// gzip is expanded only for clients that advertise neither encoding.
//
//go:embed web/*.html web/*.css web/*.js web/*.gz web/*.br
var studioAssets embed.FS

type studioSnapshot struct {
	sourceHash     [32]byte
	sourceRevision string
	dependencies   []document.PaperSourceDependency
	revision       string
	file           string
	scenario       string
	source         string
	ast            json.RawMessage
	diagnostics    []document.PaperDiagnostic
	plan           document.PaperPlan
	pages          int
	pageSummary    []document.PaperPlanPageSummary
	baseline       *studioDetachedPlan
	captures       map[string]document.PaperPlanPageSVG
}

// studioDetachedPlan is the sole cross-source baseline. PaperPlan owns no
// source bytes; the previous Studio snapshot and AST are never retained.
type studioDetachedPlan struct {
	revision    string
	scenario    string
	plan        document.PaperPlan
	pageSummary []document.PaperPlanPageSummary
}

type studioServer struct {
	file             string
	scenario         string
	sessionToken     string
	listenHost       string
	listenPort       string
	mu               sync.Mutex
	editMu           sync.RWMutex
	reviewMu         sync.Mutex
	snapshots        map[string]*studioSnapshot
	sourceHash       [32]byte
	hasSourceHash    bool
	previous         *studioDetachedPlan
	static           http.Handler
	assets           []document.PaperAssetResource
	assetCatalog     document.PaperAssetCatalog
	projectResources []paperassets.ProjectResource
	resourceManifest string
	resourceRoot     string
	history          *paperedit.Journal
}

type studioPageRailSummary struct {
	document.PaperPlanPageSummary
	Changed    bool   `json:"changed,omitempty"`
	ChangeKind string `json:"change_kind,omitempty"`
}

type studioBaselineResponse struct {
	Status           string `json:"status"`
	Revision         string `json:"revision,omitempty"`
	Scenario         string `json:"scenario,omitempty"`
	ChangedPageCount uint32 `json:"changed_page_count"`
	RemovedPageCount uint32 `json:"removed_page_count"`
}

type studioWorkspaceResponse struct {
	FormatVersion  uint16                     `json:"format_version"`
	File           string                     `json:"file"`
	Revision       string                     `json:"revision"`
	SourceRevision string                     `json:"source_revision"`
	PlanHash       string                     `json:"plan_hash,omitempty"`
	Pages          int                        `json:"pages"`
	Source         string                     `json:"source"`
	AST            json.RawMessage            `json:"ast,omitempty"`
	Diagnostics    []document.PaperDiagnostic `json:"diagnostics"`
	Scenario       string                     `json:"scenario,omitempty"`
	Preview        string                     `json:"preview_status"`
	PageRail       []studioPageRailSummary    `json:"page_rail,omitempty"`
	Baseline       studioBaselineResponse     `json:"baseline"`
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Printf("paper-studio %s\n", resolvedStudioVersion())
		return
	}
	addr := flag.String("addr", "127.0.0.1:7331", "loopback listen address")
	scenario := flag.String("scenario", "", "explicit .paper scenario")
	assetsManifest := flag.String("assets", "", "explicit JSON asset manifest")
	assetRoot := flag.String("asset-root", "", "asset root (defaults to explicit manifest directory)")
	noOpen := flag.Bool("no-open", false, "print the protected URL without opening a browser")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: paper-studio [-addr 127.0.0.1:7331] [-scenario NAME] [-no-open] FILE.paper|FILE.paperdoc")
		os.Exit(2)
	}
	if err := validateStudioListenAddress(*addr); err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	server, err := newStudioServer(flag.Arg(0), *scenario)
	if err != nil {
		log.Fatal(err)
	}
	if *assetRoot != "" && *assetsManifest == "" {
		log.Fatal("paper-studio: -asset-root requires -assets")
	}
	if *assetsManifest != "" && isStudioPaperDocument(server.file) {
		log.Fatal("paper-studio: a .paperdoc already contains its resource catalog")
	}
	if *assetsManifest != "" {
		project, loadErr := paperassets.LoadProjectManifest(*assetsManifest, *assetRoot)
		if loadErr != nil {
			log.Fatal(loadErr)
		}
		if loadErr = server.setProjectManifest(*assetsManifest, *assetRoot, project); loadErr != nil {
			log.Fatal(loadErr)
		}
	}
	if err := server.setListenerAddress(listener.Addr().String()); err != nil {
		log.Fatal(err)
	}
	httpServer := &http.Server{Handler: server.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	studioURL := fmt.Sprintf("http://%s/#token=%s", listener.Addr(), url.QueryEscape(server.sessionToken))
	fmt.Fprintf(os.Stderr, "Paper Studio: %s\n", studioURL)
	if !*noOpen {
		if err := openStudioBrowser(studioURL, runtime.GOOS); err != nil {
			fmt.Fprintf(os.Stderr, "Paper Studio: browser did not open: %v\n", err)
		}
	}
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func resolvedStudioVersion() string {
	if version != "" && version != "devel" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "devel"
}

func validateStudioListenAddress(address string) error {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return fmt.Errorf("paper-studio: invalid listen address: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("paper-studio: listen address must use an explicit loopback host")
	}
	return nil
}

func newStudioServer(file, scenario string) (*studioServer, error) {
	if strings.TrimSpace(file) == "" {
		return nil, errors.New("paper-studio: source file is required")
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	web, err := fs.Sub(studioAssets, "web")
	if err != nil {
		return nil, err
	}
	static, err := newStudioStaticHandler(web)
	if err != nil {
		return nil, err
	}
	token, err := newStudioSessionToken()
	if err != nil {
		return nil, err
	}
	server := &studioServer{file: abs, scenario: strings.TrimSpace(scenario), sessionToken: token, listenHost: "127.0.0.1", listenPort: "7331", snapshots: make(map[string]*studioSnapshot), static: static}
	if isStudioPaperDocument(abs) {
		if err := server.loadPaperDocument(); err != nil {
			return nil, err
		}
	}
	return server, nil
}

func newStudioSessionToken() (string, error) {
	raw := make([]byte, studioSessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("paper-studio: create session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *studioServer) setListenerAddress(address string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil || port == "" {
		return errors.New("paper-studio: listener address has no port")
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	s.listenHost = host
	s.listenPort = port
	return nil
}

func newStudioStaticHandler(web fs.FS) (http.Handler, error) {
	compressedWASM, err := fs.ReadFile(web, "paper-studio.wasm.gz")
	if err != nil {
		return nil, fmt.Errorf("paper-studio: read compressed WASM: %w", err)
	}
	brotliWASM, err := fs.ReadFile(web, "paper-studio.wasm.br")
	if err != nil {
		return nil, fmt.Errorf("paper-studio: read Brotli WASM: %w", err)
	}
	wasmRuntime, err := fs.ReadFile(web, "wasm_exec.js")
	if err != nil {
		return nil, fmt.Errorf("paper-studio: read WASM runtime: %w", err)
	}
	workerSource, err := fs.ReadFile(web, "wasm-renderer-worker.js")
	if err != nil {
		return nil, fmt.Errorf("paper-studio: read WASM worker: %w", err)
	}
	// The worker is served as one script so its locked-down CSP never needs to
	// grant importScripts access. This also prevents renderer code from using a
	// script fetch as an alternate same-origin communication channel.
	bundledWorker := make([]byte, 0, len(wasmRuntime)+1+len(workerSource))
	bundledWorker = append(bundledWorker, wasmRuntime...)
	bundledWorker = append(bundledWorker, '\n')
	bundledWorker = append(bundledWorker, workerSource...)
	files := http.FileServer(http.FS(web))
	var decodeOnce sync.Once
	var plainWASM []byte
	var decodeErr error
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wasm-renderer-worker.js" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Length", strconv.Itoa(len(bundledWorker)))
			w.WriteHeader(http.StatusOK)
			if r.Method == http.MethodGet {
				_, _ = w.Write(bundledWorker)
			}
			return
		}
		if r.URL.Path != "/paper-studio.wasm" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/wasm")
		w.Header().Set("Vary", "Accept-Encoding")
		body := compressedWASM
		if acceptsEncoding(r.Header.Get("Accept-Encoding"), "br") {
			body = brotliWASM
			w.Header().Set("Content-Encoding", "br")
		} else if acceptsGzip(r.Header.Get("Accept-Encoding")) {
			w.Header().Set("Content-Encoding", "gzip")
		} else {
			decodeOnce.Do(func() {
				reader, openErr := gzip.NewReader(bytes.NewReader(compressedWASM))
				if openErr != nil {
					decodeErr = openErr
					return
				}
				plainWASM, decodeErr = io.ReadAll(reader)
				closeErr := reader.Close()
				if decodeErr == nil {
					decodeErr = closeErr
				}
			})
			if decodeErr != nil {
				http.Error(w, "paper-studio: compressed WASM is invalid", http.StatusInternalServerError)
				return
			}
			body = plainWASM
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	}), nil
}

func acceptsGzip(value string) bool {
	return acceptsEncoding(value, "gzip")
}

func acceptsEncoding(value, name string) bool {
	for _, encoding := range strings.Split(value, ",") {
		parts := strings.Split(encoding, ";")
		if !strings.EqualFold(strings.TrimSpace(parts[0]), name) {
			continue
		}
		for _, parameter := range parts[1:] {
			key, rawQuality, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			quality, qualityErr := strconv.ParseFloat(strings.TrimSpace(rawQuality), 64)
			return qualityErr == nil && quality > 0
		}
		return true
	}
	return false
}

func (s *studioServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/workspace", s.handleWorkspace)
	mux.HandleFunc("/api/changes", s.handleChanges)
	mux.HandleFunc("/api/page/", s.handlePage)
	mux.HandleFunc("/api/hit", s.handleHit)
	mux.HandleFunc("/api/explain", s.handleExplain)
	mux.HandleFunc("/api/inspect", s.handleInspect)
	mux.HandleFunc("/api/delivery", s.handleDelivery)
	mux.HandleFunc("/api/export.pdf", s.handleExportPDF)
	mux.HandleFunc("/api/export.paperdoc", s.handleExportPaperDocument)
	mux.HandleFunc("/api/edit", s.handleEdit)
	mux.HandleFunc("/api/validate-edit", s.handleValidateEdit)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/resources", s.handleResources)
	mux.HandleFunc("/api/authoring", s.handleAuthoring)
	mux.HandleFunc("/api/component-preview.svg", s.handleComponentPreview)
	mux.HandleFunc("/api/review", s.handleReview)
	mux.HandleFunc("/api/review/reference", s.handleReviewReference)
	mux.Handle("/", s.static)
	return s.securityHeaders(s.requestGuard(mux))
}

func (s *studioServer) requestGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.permitsHost(r.Host) {
			http.Error(w, "paper-studio: forbidden host", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if !s.authenticatesAPIRequest(r) {
				http.Error(w, "paper-studio: forbidden API session", http.StatusForbidden)
				return
			}
			origin := r.Header.Get("Origin")
			if origin != "" && !s.permitsOrigin(origin) {
				http.Error(w, "paper-studio: forbidden origin", http.StatusForbidden)
				return
			}
			if isStudioMutation(r) && origin == "" {
				http.Error(w, "paper-studio: mutation origin is required", http.StatusForbidden)
				return
			}
			if isStudioJSONMutation(r) && !hasJSONContentType(r.Header.Get("Content-Type")) {
				http.Error(w, "paper-studio: application/json is required", http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *studioServer) permitsHost(value string) bool {
	host, port, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil || port != s.listenPort {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	listenIP := net.ParseIP(s.listenHost)
	return ip != nil && listenIP != nil && ip.IsLoopback() && ip.Equal(listenIP)
}

func (s *studioServer) permitsOrigin(value string) bool {
	origin, err := url.Parse(strings.TrimSpace(value))
	if err != nil || origin.Scheme != "http" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}
	return s.permitsHost(origin.Host)
}

func (s *studioServer) authenticatesAPIRequest(r *http.Request) bool {
	return s.validSessionToken(r.Header.Get(studioSessionHeader))
}

func (s *studioServer) validSessionToken(value string) bool {
	return len(value) == len(s.sessionToken) && subtle.ConstantTimeCompare([]byte(value), []byte(s.sessionToken)) == 1
}

func isStudioMutation(r *http.Request) bool {
	return r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
}

func isStudioJSONMutation(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch r.URL.Path {
	case "/api/hit", "/api/explain", "/api/inspect", "/api/edit", "/api/validate-edit", "/api/history", "/api/resources", "/api/review":
		return true
	default:
		return false
	}
}

func hasJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

func (s *studioServer) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy := "default-src 'self'; img-src 'self' data: blob:; style-src 'self'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
		if r.URL.Path == "/wasm-renderer-worker.js" {
			// The renderer receives a precompiled module and payloads exclusively
			// through postMessage. It has no network or child-worker authority.
			policy = "default-src 'none'; script-src 'wasm-unsafe-eval'; connect-src 'none'; child-src 'none'; worker-src 'none'"
		}
		w.Header().Set("Content-Security-Policy", policy)
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func (s *studioServer) current(ctx context.Context, requestedScenario string) (*studioSnapshot, error) {
	scenario := strings.TrimSpace(requestedScenario)
	s.mu.Lock()
	if scenario == "" {
		scenario = s.scenario
	}
	catalog := s.assetCatalog
	s.mu.Unlock()

	source, hash, err := readStudioSource(s.file)
	if err != nil {
		return nil, err
	}
	parsed := paperlang.Parse(s.file, source)
	imports := hasPaperImports(parsed.AST)

	s.mu.Lock()
	if !s.hasSourceHash || s.sourceHash != hash {
		s.previous = nil
		if prior := s.snapshots[scenario]; prior != nil && prior.pages > 0 && prior.revision != "" {
			s.previous = &studioDetachedPlan{revision: prior.revision, scenario: prior.scenario, plan: prior.plan,
				pageSummary: append([]document.PaperPlanPageSummary(nil), prior.pageSummary...)}
		}
		clear(s.snapshots)
		s.sourceHash, s.hasSourceHash = hash, true
	}
	cached := s.snapshots[scenario]
	if cached != nil && cached.sourceHash == hash && !imports {
		s.mu.Unlock()
		return cached, nil
	}
	baseline := s.previous
	s.mu.Unlock()
	if cached != nil && cached.sourceHash == hash && imports {
		dependencies := append([]document.PaperSourceDependency(nil), cached.dependencies...)
		if s.watchedSourceRevision(ctx, source, dependencies) == cached.sourceRevision {
			return cached, nil
		}
	}

	ast, astErr := parsed.AST.CanonicalJSON()
	if astErr != nil {
		return nil, astErr
	}
	var plan document.PaperPlan
	var planned document.PaperPlanResult
	resolver := s.studioImportResolver()
	if scenario == "" {
		plan, planned, err = document.PlanPaperWithAssetsAndImportsContext(ctx, s.file, source, catalog, resolver)
	} else {
		plan, planned, err = document.PlanPaperScenarioWithAssetsAndImportsContext(ctx, s.file, source, scenario, catalog, resolver)
	}
	sourceRevision := planned.SourceRevision
	if sourceRevision == "" {
		sourceRevision = studioSourceRevision(source)
	}
	revision := "source-" + hex.EncodeToString(hash[:8])
	pages := 0
	if err == nil && planned.OK() {
		revision, pages = plan.Hash(), plan.PageCount()
	}
	var pageSummary []document.PaperPlanPageSummary
	if pages > 0 {
		pageSummary, err = plan.PageSummariesWithLimits(document.PaperPlanPageSummaryLimits{
			MaxPages: studioPageRailLimit, MaxIssuesPerPage: studioPageIssueLimit,
		})
		if err != nil {
			return nil, err
		}
	}
	snapshot := &studioSnapshot{sourceHash: hash, sourceRevision: sourceRevision, dependencies: append([]document.PaperSourceDependency(nil), planned.Dependencies...),
		revision: revision, file: s.file, scenario: scenario, source: source, ast: ast,
		diagnostics: append([]document.PaperDiagnostic(nil), planned.Diagnostics...), plan: plan, pages: pages,
		pageSummary: pageSummary, baseline: baseline, captures: make(map[string]document.PaperPlanPageSVG)}

	s.mu.Lock()
	defer s.mu.Unlock()
	if cached := s.snapshots[scenario]; cached != nil && cached.sourceHash == hash && cached.sourceRevision == sourceRevision {
		return cached, nil
	} else if cached != nil && cached.pages > 0 && cached.revision != "" {
		s.previous = &studioDetachedPlan{revision: cached.revision, scenario: cached.scenario, plan: cached.plan,
			pageSummary: append([]document.PaperPlanPageSummary(nil), cached.pageSummary...)}
		snapshot.baseline = s.previous
	}
	_, alreadyCached := s.snapshots[scenario]
	cacheableImports := !imports || (err == nil && planned.OK())
	if cacheableImports && (alreadyCached || len(s.snapshots) < studioScenarioCacheLimit) && (scenario == "" || (err == nil && planned.OK())) {
		s.snapshots[scenario] = snapshot
	}
	return snapshot, nil
}

func hasPaperImports(ast paperlang.AST) bool {
	if ast.Root == nil {
		return false
	}
	for _, member := range ast.Root.Members {
		if member.Property != nil && member.Property.Name == "import" {
			return true
		}
	}
	return false
}

func studioFileImportResolver(projectFile string) document.PaperImportResolver {
	projectRootPath, rootErr := filepath.Abs(filepath.Dir(projectFile))
	projectRoot := projectRootPath
	if rootErr == nil {
		projectRoot, rootErr = filepath.EvalSymlinks(projectRootPath)
	}
	return func(ctx context.Context, importerFile, importPath string) (string, string, error) {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		if rootErr != nil {
			return "", "", fmt.Errorf("paper-studio: resolve project root: %w", rootErr)
		}
		if isStudioPaperDocument(importerFile) {
			return "", "", errors.New("paper-studio: Paper Document v1 cannot resolve external imports")
		}
		base := filepath.Dir(importerFile)
		candidate, err := filepath.Abs(filepath.Clean(filepath.Join(base, filepath.FromSlash(importPath))))
		if err != nil {
			return "", "", err
		}
		if err := requireStudioProjectPath(projectRootPath, candidate); err != nil {
			return "", "", err
		}
		file, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return "", "", err
		}
		if err := requireStudioProjectPath(projectRoot, file); err != nil {
			return "", "", err
		}
		encoded, err := readStudioImportFileContext(ctx, file)
		if err != nil {
			return "", "", err
		}
		return file, string(encoded), nil
	}
}

func readStudioImportFileContext(ctx context.Context, file string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	limit := document.PaperImportReadLimit(ctx, studioSourceLimit)
	info, err := os.Stat(file) // #nosec G304,G703 -- file is a validated source-relative Studio import.
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("paper-studio: imported source is not a regular file")
	}
	if info.Size() < 0 || info.Size() > limit {
		return nil, fmt.Errorf("imported source exceeds %d bytes", limit)
	}
	input, err := os.Open(file) // #nosec G304,G703 -- path containment and regular-file type were validated above.
	if err != nil {
		return nil, err
	}
	defer func() { _ = input.Close() }()
	openedInfo, err := input.Stat()
	if err != nil {
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, errors.New("paper-studio: imported source is not a regular file")
	}
	encoded, err := io.ReadAll(io.LimitReader(&studioContextReader{ctx: ctx, reader: input}, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(encoded)) > limit {
		return nil, fmt.Errorf("imported source exceeds %d bytes", limit)
	}
	return encoded, nil
}

type studioContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *studioContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if err == nil {
		err = r.ctx.Err()
	}
	return n, err
}

func requireStudioProjectPath(projectRoot, candidate string) error {
	relative, err := filepath.Rel(projectRoot, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("paper-studio: import path escapes the project root")
	}
	return nil
}

func readStudioSource(file string) (string, [32]byte, error) {
	if isStudioPaperDocument(file) {
		return readStudioPaperDocumentSource(file)
	}
	input, err := os.Open(file) // #nosec G304,G703 -- file is the explicit Studio source path selected by the caller.
	if err != nil {
		return "", [32]byte{}, err
	}
	defer func() { _ = input.Close() }()
	limited := io.LimitReader(input, studioSourceLimit+1)
	encoded, err := io.ReadAll(limited)
	if err != nil {
		return "", [32]byte{}, err
	}
	if len(encoded) > studioSourceLimit {
		return "", [32]byte{}, fmt.Errorf("paper-studio: source exceeds %d bytes", studioSourceLimit)
	}
	return string(encoded), sha256.Sum256(encoded), nil
}

func (s *studioServer) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()
	snapshot, err := s.current(ctx, r.URL.Query().Get("scenario"))
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	status := "plan_preview"
	if snapshot.pages == 0 {
		status = "unavailable"
	}
	pageRail, baseline := s.pageRail(snapshot)
	writeStudioJSON(w, http.StatusOK, studioWorkspaceResponse{FormatVersion: 1, File: snapshot.file, Revision: snapshot.revision,
		SourceRevision: studioSnapshotSourceRevision(snapshot),
		PlanHash:       snapshot.plan.Hash(), Pages: snapshot.pages, Source: snapshot.source, AST: snapshot.ast,
		Diagnostics: snapshot.diagnostics, Scenario: snapshot.scenario, Preview: status, PageRail: pageRail, Baseline: baseline})
}

// handleChanges keeps the browser informed without making it poll the full
// workspace response. It watches only the source digest, never sends source
// bytes, and leaves exact plan construction to the next revision-bound
// workspace request. The small server-side interval keeps this dependency-free
// while the browser receives changes as soon as the file is observed.
func (s *studioServer) handleChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeStudioError(w, http.StatusInternalServerError, errors.New("paper-studio: change stream requires streaming response support"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	snapshot, err := s.current(ctx, r.URL.Query().Get("scenario"))
	cancel()
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	lastRevision := studioSnapshotSourceRevision(snapshot)
	dependencies := append([]document.PaperSourceDependency(nil), snapshot.dependencies...)
	expectedRevision := strings.TrimSpace(r.URL.Query().Get("source_revision"))
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("X-Accel-Buffering", "no")
	_, _ = io.WriteString(w, "retry: 1000\n\n: connected\n\n")
	flusher.Flush()

	writeChanged := func(revision string) bool {
		payload, marshalErr := json.Marshal(map[string]string{
			"source_revision": revision,
			"scenario":        strings.TrimSpace(r.URL.Query().Get("scenario")),
		})
		if marshalErr != nil {
			return false
		}
		if _, writeErr := fmt.Fprintf(w, "event: changed\ndata: %s\n\n", payload); writeErr != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if expectedRevision != "" && expectedRevision != lastRevision && !writeChanged(lastRevision) {
		return
	}

	poll := time.NewTicker(studioChangePollInterval)
	defer poll.Stop()
	keepAlive := time.NewTicker(studioChangeKeepAlive)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-poll.C:
			source, _, readErr := readStudioSource(s.file)
			if readErr != nil {
				continue
			}
			revision := s.watchedSourceRevision(r.Context(), source, dependencies)
			if revision == lastRevision {
				continue
			}
			lastRevision = revision
			if !writeChanged(revision) {
				return
			}
		case <-keepAlive.C:
			if _, writeErr := io.WriteString(w, ": keep-alive\n\n"); writeErr != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *studioServer) watchedSourceRevision(ctx context.Context, source string, dependencies []document.PaperSourceDependency) string {
	current := make([]document.PaperSourceDependency, 0, len(dependencies))
	if isStudioPaperDocument(s.file) {
		packaged, _, err := readStudioPaperDocument(s.file)
		if err != nil {
			return document.PaperSourceManifestRevision(source, nil)
		}
		prefix := s.file + "!/"
		for _, dependency := range dependencies {
			if !strings.HasPrefix(dependency.File, prefix) {
				continue
			}
			value, ok := packaged.Imports[strings.TrimPrefix(dependency.File, prefix)]
			if !ok {
				continue
			}
			digest := sha256.Sum256([]byte(value))
			current = append(current, document.PaperSourceDependency{File: dependency.File, Digest: hex.EncodeToString(digest[:])})
		}
		return document.PaperSourceManifestRevision(source, current)
	}
	for _, dependency := range dependencies {
		value, err := readStudioImportFileContext(ctx, dependency.File)
		if err != nil {
			continue
		}
		digest := sha256.Sum256(value)
		current = append(current, document.PaperSourceDependency{File: dependency.File, Digest: hex.EncodeToString(digest[:])})
	}
	return document.PaperSourceManifestRevision(source, current)
}

func (s *studioServer) pageRail(snapshot *studioSnapshot) ([]studioPageRailSummary, studioBaselineResponse) {
	previous := snapshot.baseline
	rows := make([]studioPageRailSummary, len(snapshot.pageSummary))
	for index, summary := range snapshot.pageSummary {
		rows[index].PaperPlanPageSummary = summary
	}
	if snapshot.pages == 0 {
		baseline := studioBaselineResponse{Status: "current_unavailable"}
		if previous != nil {
			baseline.Revision, baseline.Scenario = previous.revision, previous.scenario
		}
		return rows, baseline
	}
	if previous == nil {
		return rows, studioBaselineResponse{Status: "none"}
	}
	baseline := studioBaselineResponse{Status: "scenario_mismatch", Revision: previous.revision, Scenario: previous.scenario}
	if previous.scenario != snapshot.scenario {
		return rows, baseline
	}
	baseline.Status = "available"
	before := make(map[uint32]document.PaperPlanPageSummary, len(previous.pageSummary))
	for _, summary := range previous.pageSummary {
		before[summary.Page] = summary
	}
	for index := range rows {
		prior, exists := before[rows[index].Page]
		delete(before, rows[index].Page)
		switch {
		case !exists:
			rows[index].Changed, rows[index].ChangeKind = true, "added"
		case prior.ContentHash != rows[index].ContentHash:
			rows[index].Changed, rows[index].ChangeKind = true, "modified"
		}
		if rows[index].Changed {
			baseline.ChangedPageCount++
		}
	}
	baseline.RemovedPageCount = uint32(len(before)) // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
	baseline.ChangedPageCount += baseline.RemovedPageCount
	return rows, baseline
}

func (s *studioServer) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/page/")
	kind := "display"
	if strings.HasSuffix(name, ".geometry.svg") {
		kind, name = "geometry", strings.TrimSuffix(name, ".geometry.svg")
	} else if strings.HasSuffix(name, ".render") {
		kind, name = "wasm", strings.TrimSuffix(name, ".render")
	} else if strings.HasSuffix(name, ".svg") {
		name = strings.TrimSuffix(name, ".svg")
	} else {
		http.NotFound(w, r)
		return
	}
	page64, err := strconv.ParseUint(name, 10, 32)
	if err != nil || page64 == 0 {
		writeStudioError(w, http.StatusBadRequest, errors.New("paper-studio: invalid page"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()
	snapshot, err := s.current(ctx, r.URL.Query().Get("scenario"))
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if r.URL.Query().Get("revision") != snapshot.revision {
		writeStudioError(w, http.StatusConflict, errors.New("paper-studio: stale preview revision"))
		return
	}
	if int(page64) > snapshot.pages {
		http.NotFound(w, r)
		return
	}
	if kind == "wasm" {
		renderRequest := document.DefaultPaperPlanWebRenderRequest(uint32(page64))
		if rawDPI := r.URL.Query().Get("dpi"); rawDPI != "" {
			dpi, dpiErr := strconv.ParseUint(rawDPI, 10, 32)
			if dpiErr != nil || dpi < 36 || dpi > 600 {
				writeStudioError(w, http.StatusBadRequest, errors.New("paper-studio: render DPI must be between 36 and 600"))
				return
			}
			renderRequest.DPI = uint32(dpi)
		}
		payload, payloadErr := snapshot.plan.WebDisplayRenderPayload(ctx, renderRequest)
		if payloadErr != nil {
			writeStudioError(w, http.StatusUnprocessableEntity, payloadErr)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.paperrune.display-render")
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		w.Header().Set("ETag", `"`+snapshot.revision+`-wasm-`+name+`-`+strconv.FormatUint(uint64(renderRequest.DPI), 10)+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	}
	cacheKey := kind + ":" + name
	s.mu.Lock()
	capture, ok := snapshot.captures[cacheKey]
	if !ok {
		if kind == "geometry" {
			capture, err = snapshot.plan.CaptureGeometryPageSVG(uint32(page64))
		} else {
			capture, err = snapshot.plan.CaptureDisplayPageSVG(ctx, uint32(page64), nil)
		}
		if err == nil {
			snapshot.captures[cacheKey] = capture
		}
	}
	s.mu.Unlock()
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+snapshot.revision+`-`+kind+`-`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(capture.SVG)
}

type studioHitRequest struct {
	Revision string `json:"revision"`
	Scenario string `json:"scenario,omitempty"`
	Page     uint32 `json:"page"`
	X        int64  `json:"x_fixed"`
	Y        int64  `json:"y_fixed"`
}

func (s *studioServer) handleHit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request studioHitRequest
	if err := decodeStudioJSON(r, &request); err != nil {
		writeStudioError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()
	snapshot, err := s.current(ctx, request.Scenario)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if request.Revision != snapshot.revision {
		writeStudioError(w, http.StatusConflict, errors.New("paper-studio: stale preview revision"))
		return
	}
	result, err := snapshot.plan.HitTest(request.Page, request.X, request.Y)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeStudioRawJSON(w, http.StatusOK, result.JSON())
}

type studioExplainRequest struct {
	Revision string                     `json:"revision"`
	Scenario string                     `json:"scenario,omitempty"`
	Selector document.PaperPlanSelector `json:"selector"`
}

// studioInspectRequest selects the bounded exact-plan evidence needed by the
// page inspector. It deliberately contains no presentation or browser-layout
// inputs: all coordinates, roles, reading indexes, regions, and break causes
// come from the immutable plan named by Revision.
type studioInspectRequest struct {
	Revision string `json:"revision"`
	Scenario string `json:"scenario,omitempty"`
	Page     uint32 `json:"page"`
}

func (s *studioServer) handleInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request studioInspectRequest
	if err := decodeStudioJSON(r, &request); err != nil {
		writeStudioError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()
	snapshot, err := s.current(ctx, request.Scenario)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if request.Revision != snapshot.revision {
		writeStudioError(w, http.StatusConflict, errors.New("paper-studio: stale preview revision"))
		return
	}
	if request.Page == 0 || int(request.Page) > snapshot.pages {
		writeStudioError(w, http.StatusBadRequest, errors.New("paper-studio: invalid inspection page"))
		return
	}
	result, err := snapshot.plan.ExplainContext(ctx, []document.PaperPlanSelector{{
		Page: request.Page, MaxResults: studioInspectionLimit,
	}}, 1, studioJSONLimit, 2<<20)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeStudioRawJSON(w, http.StatusOK, result.JSON())
}

func (s *studioServer) handleExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request studioExplainRequest
	if err := decodeStudioJSON(r, &request); err != nil {
		writeStudioError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()
	snapshot, err := s.current(ctx, request.Scenario)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if request.Revision != snapshot.revision {
		writeStudioError(w, http.StatusConflict, errors.New("paper-studio: stale preview revision"))
		return
	}
	request.Selector.MaxResults = 64
	result, err := snapshot.plan.ExplainContext(ctx, []document.PaperPlanSelector{request.Selector}, 1, studioJSONLimit, 1<<20)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeStudioRawJSON(w, http.StatusOK, result.JSON())
}

func decodeStudioJSON(r *http.Request, target any) error {
	return decodeStudioJSONWithLimit(r, target, studioJSONLimit)
}

func decodeStudioJSONWithLimit(r *http.Request, target any, limit int64) error {
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(io.LimitReader(r.Body, limit+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("paper-studio: invalid request: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("paper-studio: request must contain one JSON value")
	}
	return nil
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeStudioError(w, http.StatusMethodNotAllowed, errors.New("paper-studio: method not allowed"))
}

func writeStudioJSON(w http.ResponseWriter, status int, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		writeStudioError(w, http.StatusInternalServerError, err)
		return
	}
	writeStudioRawJSON(w, status, encoded)
}

func writeStudioRawJSON(w http.ResponseWriter, status int, encoded []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

func writeStudioError(w http.ResponseWriter, status int, err error) {
	writeStudioJSON(w, status, struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}{false, err.Error()})
}
