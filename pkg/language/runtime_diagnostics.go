package language

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	RuntimeDiagnosticsSource        = "hyperbricks-runtime"
	maxRuntimeStatusUncheckedRoutes = 256
)

// RuntimeDiagnosticsClient reads the runtime's complete current-diagnostics
// snapshot. Implementations must not turn transport failures into source
// diagnostics.
type RuntimeDiagnosticsClient interface {
	FetchCurrent(context.Context) (RuntimeDiagnosticsSnapshot, error)
}

// RuntimeDiagnosticsSink is implemented by the language-server transport. A
// replacement is authoritative for every file in Diagnostics and every file
// in ClearedFiles. Route coverage is published separately so an unvisited
// route never appears as a source problem.
type RuntimeDiagnosticsSink interface {
	ReplaceRuntimeDiagnostics(context.Context, RuntimeDiagnosticsUpdate) error
	PublishRuntimeStatus(context.Context, RuntimeDiagnosticsStatus) error
}

// RuntimeDiagnosticsSnapshot is the JSON contract returned by
// /__hyperbricks/render-diagnostics?view=current.
type RuntimeDiagnosticsSnapshot struct {
	Records         []RuntimeDiagnosticsRecord `json:"records"`
	Generation      uint64                     `json:"generation"`
	CheckedRoutes   int                        `json:"checked_routes"`
	TotalRoutes     int                        `json:"total_routes"`
	UncheckedRoutes []string                   `json:"unchecked_routes"`
	EvictedContexts int                        `json:"evicted_contexts"`
}

type RuntimeDiagnosticsRecord struct {
	RequestID  string                    `json:"request_id"`
	Route      string                    `json:"route"`
	CreatedAt  time.Time                 `json:"created_at"`
	Errors     []RuntimeDiagnosticsIssue `json:"errors"`
	ContextID  string                    `json:"context_id,omitempty"`
	Method     string                    `json:"method,omitempty"`
	Generation uint64                    `json:"generation"`
}

type RuntimeDiagnosticsIssue struct {
	Hash           string `json:"hash"`
	Type           string `json:"type"`
	File           string `json:"file"`
	Path           string `json:"path"`
	Key            string `json:"key"`
	Err            string `json:"err"`
	Level          string `json:"level,omitempty"`
	Rejected       bool   `json:"rejected,omitempty"`
	Line           int    `json:"line,omitempty"`
	Column         int    `json:"column,omitempty"`
	Resource       string `json:"resource,omitempty"`
	ResourceLine   int    `json:"resource_line,omitempty"`
	ResourceColumn int    `json:"resource_column,omitempty"`
	Phase          string `json:"phase,omitempty"`
}

// RuntimeDiagnosticSeverity uses the Language Server Protocol numeric values,
// while keeping this package independent of a particular LSP implementation.
type RuntimeDiagnosticSeverity int

const (
	RuntimeSeverityError       RuntimeDiagnosticSeverity = 1
	RuntimeSeverityWarning     RuntimeDiagnosticSeverity = 2
	RuntimeSeverityInformation RuntimeDiagnosticSeverity = 3
)

type RuntimeEditorPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type RuntimeEditorRange struct {
	Start RuntimeEditorPosition `json:"start"`
	End   RuntimeEditorPosition `json:"end"`
}

type RuntimeRelatedLocation struct {
	File    string             `json:"file"`
	Range   RuntimeEditorRange `json:"range"`
	Message string             `json:"message"`
}

type RuntimeRequestContext struct {
	RequestID  string `json:"requestId"`
	ContextID  string `json:"contextId,omitempty"`
	Route      string `json:"route"`
	Method     string `json:"method,omitempty"`
	Phase      string `json:"phase,omitempty"`
	Generation uint64 `json:"generation"`
}

// RuntimeEditorDiagnostic contains editor-neutral fields that an LSP transport
// can map directly to Diagnostic and DiagnosticRelatedInformation values.
type RuntimeEditorDiagnostic struct {
	Source        string                    `json:"source"`
	Code          string                    `json:"code"`
	Message       string                    `json:"message"`
	StatusMessage string                    `json:"-"`
	Severity      RuntimeDiagnosticSeverity `json:"severity"`
	File          string                    `json:"file,omitempty"`
	Range         RuntimeEditorRange        `json:"range"`
	ComponentType string                    `json:"componentType,omitempty"`
	ComponentPath string                    `json:"componentPath,omitempty"`
	Key           string                    `json:"key,omitempty"`
	Phase         string                    `json:"phase,omitempty"`
	Related       []RuntimeRelatedLocation  `json:"related,omitempty"`
	Contexts      []RuntimeRequestContext   `json:"contexts"`
}

// RuntimeDiagnosticsUpdate replaces the previous runtime diagnostic set. The
// sink publishes empty diagnostic arrays for ClearedFiles, which is what makes
// a successful retry remove a previous Problems entry.
type RuntimeDiagnosticsUpdate struct {
	Generation        uint64                               `json:"generation"`
	GenerationChanged bool                                 `json:"generationChanged"`
	Diagnostics       map[string][]RuntimeEditorDiagnostic `json:"diagnostics"`
	Unmapped          []RuntimeEditorDiagnostic            `json:"unmapped,omitempty"`
	ClearedFiles      []string                             `json:"clearedFiles,omitempty"`
}

// RuntimeDiagnosticsStatus is deliberately separate from diagnostics. In
// particular, UncheckedRoutes communicates incomplete runtime coverage without
// creating false warnings in source files.
type RuntimeDiagnosticsStatus struct {
	Connected       bool          `json:"connected"`
	Generation      uint64        `json:"generation"`
	ErrorCount      int           `json:"errorCount"`
	CheckedRoutes   int           `json:"checkedRoutes"`
	TotalRoutes     int           `json:"totalRoutes"`
	UncheckedRoutes []string      `json:"uncheckedRoutes"`
	EvictedContexts int           `json:"evictedContexts"`
	UnmappedIssues  []string      `json:"unmappedIssues,omitempty"`
	LastError       string        `json:"lastError,omitempty"`
	NextRetry       time.Duration `json:"nextRetry,omitempty"`
	RuntimeURL      string        `json:"runtimeUrl,omitempty"`
	ErrorsURL       string        `json:"errorsUrl,omitempty"`
}

// RuntimeDiagnosticsState translates complete runtime snapshots and tracks the
// files that must be cleared on the next replacement.
type RuntimeDiagnosticsState struct {
	mu            sync.Mutex
	workspaceRoot string
	configPath    string
	generation    uint64
	hasGeneration bool
	files         map[string]struct{}
}

func NewRuntimeDiagnosticsState(workspaceRoot, configPath string) *RuntimeDiagnosticsState {
	workspaceRoot = cleanWorkspaceRoot(workspaceRoot)
	configPath, _ = resolveRuntimeConfigPath(workspaceRoot, configPath)
	return &RuntimeDiagnosticsState{
		workspaceRoot: workspaceRoot,
		configPath:    configPath,
		files:         make(map[string]struct{}),
	}
}

func (state *RuntimeDiagnosticsState) Replace(snapshot RuntimeDiagnosticsSnapshot) RuntimeDiagnosticsUpdate {
	state.mu.Lock()
	defer state.mu.Unlock()

	diagnostics, unmapped := mapRuntimeDiagnostics(snapshot, state.workspaceRoot, state.configPath)
	nextFiles := make(map[string]struct{}, len(diagnostics))
	for file := range diagnostics {
		nextFiles[file] = struct{}{}
	}
	cleared := differenceRuntimeFiles(state.files, nextFiles)
	generationChanged := state.hasGeneration && state.generation != snapshot.Generation
	state.generation = snapshot.Generation
	state.hasGeneration = true
	state.files = nextFiles

	return RuntimeDiagnosticsUpdate{
		Generation:        snapshot.Generation,
		GenerationChanged: generationChanged,
		Diagnostics:       diagnostics,
		Unmapped:          unmapped,
		ClearedFiles:      cleared,
	}
}

func (state *RuntimeDiagnosticsState) Clear() RuntimeDiagnosticsUpdate {
	state.mu.Lock()
	defer state.mu.Unlock()

	cleared := make([]string, 0, len(state.files))
	for file := range state.files {
		cleared = append(cleared, file)
	}
	sort.Strings(cleared)
	update := RuntimeDiagnosticsUpdate{
		Generation:        state.generation,
		GenerationChanged: state.hasGeneration,
		Diagnostics:       make(map[string][]RuntimeEditorDiagnostic),
		ClearedFiles:      cleared,
	}
	state.generation = 0
	state.hasGeneration = false
	state.files = make(map[string]struct{})
	return update
}

func mapRuntimeDiagnostics(snapshot RuntimeDiagnosticsSnapshot, workspaceRoot, configPath string) (map[string][]RuntimeEditorDiagnostic, []RuntimeEditorDiagnostic) {
	diagnostics := make(map[string][]RuntimeEditorDiagnostic)
	var unmapped []RuntimeEditorDiagnostic
	type diagnosticIndex struct {
		file  string
		index int
	}
	seen := make(map[string]diagnosticIndex)

	for _, record := range snapshot.Records {
		// A complete snapshot must not resurrect a record retained from a
		// different source generation.
		if record.Generation != 0 && snapshot.Generation != 0 && record.Generation != snapshot.Generation {
			continue
		}
		for _, issue := range record.Errors {
			diagnostic := runtimeEditorDiagnostic(record, issue, workspaceRoot, configPath)
			key := diagnostic.File + "\x00" + diagnostic.Code
			if previous, ok := seen[key]; ok {
				if previous.file == "" {
					unmapped[previous.index].Contexts = appendRuntimeContext(unmapped[previous.index].Contexts, diagnostic.Contexts[0])
				} else {
					item := diagnostics[previous.file][previous.index]
					item.Contexts = appendRuntimeContext(item.Contexts, diagnostic.Contexts[0])
					diagnostics[previous.file][previous.index] = item
				}
				continue
			}
			if diagnostic.File == "" {
				seen[key] = diagnosticIndex{index: len(unmapped)}
				unmapped = append(unmapped, diagnostic)
				continue
			}
			seen[key] = diagnosticIndex{file: diagnostic.File, index: len(diagnostics[diagnostic.File])}
			diagnostics[diagnostic.File] = append(diagnostics[diagnostic.File], diagnostic)
		}
	}

	for file := range diagnostics {
		sort.SliceStable(diagnostics[file], func(i, j int) bool {
			left, right := diagnostics[file][i], diagnostics[file][j]
			if left.Range.Start.Line != right.Range.Start.Line {
				return left.Range.Start.Line < right.Range.Start.Line
			}
			if left.Range.Start.Character != right.Range.Start.Character {
				return left.Range.Start.Character < right.Range.Start.Character
			}
			return left.Code < right.Code
		})
	}
	return diagnostics, unmapped
}

func runtimeEditorDiagnostic(record RuntimeDiagnosticsRecord, issue RuntimeDiagnosticsIssue, workspaceRoot, configPath string) RuntimeEditorDiagnostic {
	file := ""
	if strings.TrimSpace(issue.File) == "__config" {
		file, _ = resolveRuntimeConfigPath(workspaceRoot, configPath)
	} else {
		file, _ = resolveRuntimePath(workspaceRoot, issue.File)
	}
	message := sanitizeRuntimeDiagnosticText(issue.Err, 2048)
	if message == "" {
		message = "HyperBricks runtime failure"
	}
	var visibleContext []string
	if route := sanitizeRuntimeRoute(record.Route); route != "" {
		if method := safeRuntimeStatusLabel(strings.ToUpper(strings.TrimSpace(record.Method)), 32); method != "" {
			visibleContext = append(visibleContext, strings.ToUpper(method)+" "+route)
		} else {
			visibleContext = append(visibleContext, "route "+route)
		}
	}
	if requestID := sanitizeRuntimeDiagnosticText(record.RequestID, 160); requestID != "" {
		visibleContext = append(visibleContext, "request "+requestID)
	}
	componentType := sanitizeRuntimeDiagnosticText(issue.Type, 160)
	componentPath := sanitizeRuntimeDiagnosticText(issue.Path, 320)
	key := sanitizeRuntimeDiagnosticText(issue.Key, 160)
	phase := sanitizeRuntimeDiagnosticText(issue.Phase, 160)
	if componentPath != "" {
		visibleContext = append(visibleContext, "component "+componentPath)
	}
	if phase != "" {
		visibleContext = append(visibleContext, "phase "+phase)
	}
	if len(visibleContext) > 0 {
		message += " [" + strings.Join(visibleContext, "; ") + "]"
	}
	code := safeRuntimeStatusLabel(issue.Hash, 96)
	if code == "" {
		code = runtimeIssueCode(issue)
	}
	diagnostic := RuntimeEditorDiagnostic{
		Source:        RuntimeDiagnosticsSource,
		Code:          code,
		Message:       message,
		StatusMessage: sanitizeRuntimeStatusText(issue.Err, 320),
		Severity:      runtimeIssueSeverity(issue),
		File:          file,
		Range:         runtimeEditorSourceRange(file, issue.Line, issue.Column),
		ComponentType: componentType,
		ComponentPath: componentPath,
		Key:           key,
		Phase:         phase,
		Contexts: []RuntimeRequestContext{{
			RequestID:  sanitizeRuntimeDiagnosticText(record.RequestID, 160),
			ContextID:  sanitizeRuntimeDiagnosticText(record.ContextID, 160),
			Route:      sanitizeRuntimeRoute(record.Route),
			Method:     safeRuntimeStatusLabel(strings.ToUpper(strings.TrimSpace(record.Method)), 32),
			Phase:      phase,
			Generation: record.Generation,
		}},
	}
	if resource, ok := resolveRuntimePath(workspaceRoot, issue.Resource); ok {
		diagnostic.Related = []RuntimeRelatedLocation{{
			File:    resource,
			Range:   runtimeEditorResourceRange(resource, issue.ResourceLine, issue.ResourceColumn),
			Message: "Related runtime resource",
		}}
	}
	return diagnostic
}

func runtimeIssueSeverity(issue RuntimeDiagnosticsIssue) RuntimeDiagnosticSeverity {
	if issue.Rejected {
		return RuntimeSeverityError
	}
	switch strings.ToUpper(strings.TrimSpace(issue.Level)) {
	case "WARNING", "WARN":
		return RuntimeSeverityWarning
	case "INFO", "NOTICE":
		return RuntimeSeverityInformation
	default:
		return RuntimeSeverityError
	}
}

func runtimeEditorRange(line, column int) RuntimeEditorRange {
	if line > 0 {
		line--
	} else {
		line = 0
	}
	if column > 0 {
		column--
	} else {
		column = 0
	}
	return RuntimeEditorRange{
		Start: RuntimeEditorPosition{Line: line, Character: column},
		End:   RuntimeEditorPosition{Line: line, Character: column + 1},
	}
}

// runtimeEditorSourceRange converts the YAML parser's one-based Unicode
// codepoint column to the UTF-16 character units required by LSP. If the
// already-confined saved source cannot be read, retain the producer position
// instead of inventing a different anchor.
func runtimeEditorSourceRange(file string, line, column int) RuntimeEditorRange {
	fallback := runtimeEditorRange(line, column)
	if file == "" || line <= 0 || column <= 0 {
		return fallback
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return fallback
	}
	lines := splitLines(string(content))
	lineIndex := line - 1
	if lineIndex < 0 || lineIndex >= len(lines) {
		return fallback
	}
	runeColumn := column - 1
	start := utf16ColumnForRuneColumn(lines[lineIndex], runeColumn)
	end := utf16ColumnForRuneColumn(lines[lineIndex], runeColumn+1)
	if end <= start {
		end = start + 1
	}
	return RuntimeEditorRange{
		Start: RuntimeEditorPosition{Line: lineIndex, Character: start},
		End:   RuntimeEditorPosition{Line: lineIndex, Character: end},
	}
}

// runtimeEditorResourceRange converts the one-based byte columns emitted by
// Goja and Go templates to the UTF-16 character units required by LSP. The
// caller supplies only a resource path already confined by resolveRuntimePath.
// If the saved resource cannot be read, is not valid UTF-8, or the producer
// coordinate is not a valid byte boundary, retain the original position rather
// than inventing a different anchor.
func runtimeEditorResourceRange(file string, line, column int) RuntimeEditorRange {
	fallback := runtimeEditorRange(line, column)
	if file == "" || line <= 0 || column <= 0 {
		return fallback
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return fallback
	}
	lines := splitLines(string(content))
	lineIndex := line - 1
	if lineIndex < 0 || lineIndex >= len(lines) {
		return fallback
	}
	resourceLine := lines[lineIndex]
	byteColumn := column - 1
	if !utf8.ValidString(resourceLine) || byteColumn < 0 || byteColumn > len(resourceLine) ||
		(byteColumn < len(resourceLine) && !utf8.RuneStart(resourceLine[byteColumn])) {
		return fallback
	}
	start := utf16Length(resourceLine[:byteColumn])
	end := start + 1
	if byteColumn < len(resourceLine) {
		character, _ := utf8.DecodeRuneInString(resourceLine[byteColumn:])
		end = start + utf16Length(string(character))
	}
	return RuntimeEditorRange{
		Start: RuntimeEditorPosition{Line: lineIndex, Character: start},
		End:   RuntimeEditorPosition{Line: lineIndex, Character: end},
	}
}

func runtimeIssueCode(issue RuntimeDiagnosticsIssue) string {
	value := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00%d\x00%s\x00%d\x00%d\x00%s",
		issue.Type, issue.File, issue.Path, issue.Key, issue.Err, issue.Level, issue.Line, issue.Column,
		issue.Resource, issue.ResourceLine, issue.ResourceColumn, issue.Phase)
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:12])
}

func appendRuntimeContext(contexts []RuntimeRequestContext, candidate RuntimeRequestContext) []RuntimeRequestContext {
	for _, existing := range contexts {
		if existing == candidate {
			return contexts
		}
	}
	return append(contexts, candidate)
}

func resolveRuntimePath(workspaceRoot, name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || workspaceRoot == "" || strings.ContainsRune(name, '\x00') {
		return "", false
	}
	path := filepath.Clean(filepath.FromSlash(name))
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspaceRoot, path)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	if !pathWithin(workspaceRoot, absPath) {
		return "", false
	}
	resolvedRoot, err := resolvePathWithExistingAncestor(workspaceRoot)
	if err != nil {
		return "", false
	}
	resolvedPath, err := resolvePathWithExistingAncestor(absPath)
	if err != nil || !pathWithin(resolvedRoot, resolvedPath) {
		return "", false
	}
	return absPath, true
}

// resolveRuntimeConfigPath is stricter than ordinary runtime source mapping.
// The __config marker is not a filename: it may point only at the selected,
// existing package profile after resolving symlinks inside the module.
func resolveRuntimeConfigPath(workspaceRoot, name string) (string, bool) {
	if workspaceRoot == "" {
		return "", false
	}
	candidate, ok := resolveRuntimePath(workspaceRoot, name)
	if !ok {
		return "", false
	}
	info, err := os.Stat(candidate)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	resolvedRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return "", false
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return candidate, true
}

var (
	runtimeStatusURLPattern              = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"']+`)
	runtimeStatusAuthorizationPattern    = regexp.MustCompile(`(?i)\b(authorization)\s*[:=]\s*(?:(?:bearer|basic)\s+)?(?:"[^"]*"|'[^']*'|[^\s,;]+)`)
	runtimeStatusCredentialPattern       = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[-_]?key|access[-_]?key)\s*[:=]\s*(?:"[^"]*"|'[^']*'|[^\s,;]+)`)
	runtimeStatusSchemeCredentialPattern = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[a-z0-9+/._~=-]+`)
	runtimeStatusUNCPathPattern          = regexp.MustCompile(`(?i)(^|[\s("'=])\\\\[^\\\s:;,)\]}]+\\[^\\\s:;,)\]}]+(?:\\[^\\\s:;,)\]}]+)*`)
	runtimeStatusWindowsPathPattern      = regexp.MustCompile(`(?i)(^|[\s("'=])[a-z]:\\(?:[^\\\s:;,)\]}]+\\)+[^\\\s:;,)\]}]+`)
	runtimeStatusAbsolutePathPattern     = regexp.MustCompile(`(^|[\s("'=])(?:/[^/\s:;,)\]}]+){2,}`)
	runtimeStatusRelativePathPattern     = regexp.MustCompile(`(?i)(^|[\s("'=])(?:\.\.?[/\\])?(?:[^/\\\s:;,)\]}]+[/\\])+[^/\\\s:;,)\]}]+\.(?:hyperbricks\.ya?ml|ya?ml|json|html?|js|ts|css|go|tmpl|txt)`)
	runtimeStatusFilenamePattern         = regexp.MustCompile(`(?i)(^|[\s("'=])[^/\\\s:;,)\]}]+\.(?:hyperbricks\.ya?ml|ya?ml|json|html?|js|ts|css|go|tmpl|txt)`)
	runtimeStatusLabelPattern            = regexp.MustCompile(`^[A-Za-z0-9_.:<>{}\[\]-]+$`)
)

func runtimeUnmappedIssueSummaries(diagnostics []RuntimeEditorDiagnostic) []string {
	if len(diagnostics) == 0 {
		return nil
	}
	summaries := make([]string, 0, len(diagnostics))
	seen := make(map[string]struct{}, len(diagnostics))
	for _, diagnostic := range diagnostics {
		message := strings.TrimSpace(diagnostic.StatusMessage)
		if message == "" {
			message = "HyperBricks runtime issue without a workspace source location"
		}
		details := make([]string, 0, 4)
		if code := safeRuntimeStatusLabel(diagnostic.Code, 96); code != "" {
			details = append(details, "code "+code)
		}
		if componentType := safeRuntimeStatusLabel(diagnostic.ComponentType, 64); componentType != "" {
			details = append(details, "type "+componentType)
		}
		if componentPath := safeRuntimeStatusLabel(diagnostic.ComponentPath, 160); componentPath != "" {
			details = append(details, "component "+componentPath)
		}
		if phase := safeRuntimeStatusLabel(diagnostic.Phase, 64); phase != "" {
			details = append(details, "phase "+phase)
		}
		if len(details) != 0 {
			message += " (" + strings.Join(details, "; ") + ")"
		}
		message = truncateRuntimeStatusText(message, 512)
		if _, duplicate := seen[message]; duplicate {
			continue
		}
		seen[message] = struct{}{}
		summaries = append(summaries, message)
	}
	sort.Strings(summaries)
	return summaries
}

func sanitizeRuntimeStatusText(value string, limit int) string {
	value = sanitizeRuntimeDiagnosticText(value, 0)
	value = runtimeStatusUNCPathPattern.ReplaceAllString(value, `${1}[path]`)
	value = runtimeStatusWindowsPathPattern.ReplaceAllString(value, `${1}[path]`)
	value = runtimeStatusAbsolutePathPattern.ReplaceAllString(value, `${1}[path]`)
	value = runtimeStatusRelativePathPattern.ReplaceAllString(value, `${1}[path]`)
	value = runtimeStatusFilenamePattern.ReplaceAllString(value, `${1}[path]`)
	value = strings.Join(strings.Fields(value), " ")
	return truncateRuntimeStatusText(value, limit)
}

func sanitizeRuntimeDiagnosticsStatus(status RuntimeDiagnosticsStatus) RuntimeDiagnosticsStatus {
	status.LastError = sanitizeRuntimeStatusText(status.LastError, 512)
	if status.UncheckedRoutes != nil {
		routes := make([]string, 0, len(status.UncheckedRoutes))
		seen := make(map[string]struct{}, len(status.UncheckedRoutes))
		for _, route := range status.UncheckedRoutes {
			sanitized := sanitizeRuntimeRoute(route)
			if sanitized == "" {
				sanitized = "[redacted route]"
			}
			if _, duplicate := seen[sanitized]; duplicate {
				continue
			}
			seen[sanitized] = struct{}{}
			routes = append(routes, sanitized)
		}
		sort.Strings(routes)
		if len(routes) > maxRuntimeStatusUncheckedRoutes {
			routes = append([]string(nil), routes[:maxRuntimeStatusUncheckedRoutes]...)
		}
		status.UncheckedRoutes = routes
	}
	if status.UnmappedIssues != nil {
		issues := make([]string, 0, len(status.UnmappedIssues))
		for _, issue := range status.UnmappedIssues {
			if sanitized := sanitizeRuntimeStatusText(issue, 512); sanitized != "" {
				issues = append(issues, sanitized)
			}
		}
		status.UnmappedIssues = issues
	}
	return status
}

func sanitizeRuntimeDiagnosticText(value string, limit int) string {
	value = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, value)
	value = runtimeStatusURLPattern.ReplaceAllStringFunc(value, sanitizeRuntimeStatusURL)
	value = runtimeStatusAuthorizationPattern.ReplaceAllString(value, `${1}=[redacted]`)
	value = runtimeStatusCredentialPattern.ReplaceAllString(value, `${1}=[redacted]`)
	value = runtimeStatusSchemeCredentialPattern.ReplaceAllString(value, `${1} [redacted]`)
	value = strings.Join(strings.Fields(value), " ")
	return truncateRuntimeStatusText(value, limit)
}

func sanitizeRuntimeRoute(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil {
		path := parsed.EscapedPath()
		if path == "" {
			path = parsed.Path
		}
		return sanitizeRuntimeDiagnosticText(path, 320)
	}
	if index := strings.IndexAny(value, "?#"); index >= 0 {
		value = value[:index]
	}
	return sanitizeRuntimeDiagnosticText(value, 320)
}

func sanitizeRuntimeStatusURL(value string) string {
	trimmed := strings.TrimRight(value, ".,;:!?)]}")
	suffix := strings.TrimPrefix(value, trimmed)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "[redacted URL]" + suffix
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String() + suffix
}

func safeRuntimeStatusLabel(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > limit || !runtimeStatusLabelPattern.MatchString(value) {
		return ""
	}
	return value
}

func truncateRuntimeStatusText(value string, limit int) string {
	characters := []rune(value)
	if limit <= 0 || len(characters) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}
	return string(characters[:limit-1]) + "…"
}

func cleanWorkspaceRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return abs
}

func differenceRuntimeFiles(previous, current map[string]struct{}) []string {
	cleared := make([]string, 0)
	for file := range previous {
		if _, ok := current[file]; !ok {
			cleared = append(cleared, file)
		}
	}
	sort.Strings(cleared)
	return cleared
}

type RuntimePollOptions struct {
	PollInterval   time.Duration
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

func (options RuntimePollOptions) normalized() RuntimePollOptions {
	if options.PollInterval <= 0 {
		options.PollInterval = time.Second
	}
	if options.InitialBackoff <= 0 {
		options.InitialBackoff = time.Second
	}
	if options.MaxBackoff < options.InitialBackoff {
		options.MaxBackoff = 30 * time.Second
	}
	return options
}

type RuntimeDiagnosticsBridge struct {
	client  RuntimeDiagnosticsClient
	sink    RuntimeDiagnosticsSink
	state   *RuntimeDiagnosticsState
	options RuntimePollOptions

	mu   sync.Mutex
	wait func(context.Context, time.Duration) error
}

func NewRuntimeDiagnosticsBridge(client RuntimeDiagnosticsClient, sink RuntimeDiagnosticsSink, workspaceRoot, configPath string, options RuntimePollOptions) (*RuntimeDiagnosticsBridge, error) {
	if client == nil {
		return nil, fmt.Errorf("runtime diagnostics client is required")
	}
	if sink == nil {
		return nil, fmt.Errorf("runtime diagnostics sink is required")
	}
	return &RuntimeDiagnosticsBridge{
		client:  client,
		sink:    sink,
		state:   NewRuntimeDiagnosticsState(workspaceRoot, configPath),
		options: options.normalized(),
		wait:    waitForRuntimePoll,
	}, nil
}

// Refresh performs one full-snapshot replacement.
func (bridge *RuntimeDiagnosticsBridge) Refresh(ctx context.Context) error {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()

	snapshot, err := bridge.client.FetchCurrent(ctx)
	if err != nil {
		return err
	}
	update := bridge.state.Replace(snapshot)
	if err := bridge.sink.ReplaceRuntimeDiagnostics(ctx, update); err != nil {
		return fmt.Errorf("replace runtime diagnostics: %w", err)
	}
	if err := bridge.sink.PublishRuntimeStatus(ctx, runtimeStatusFromSnapshot(snapshot, update)); err != nil {
		return fmt.Errorf("publish runtime diagnostics status: %w", err)
	}
	return nil
}

// Run polls until ctx is cancelled. Transport failures only change connection
// status and increase the retry delay; they never become editor diagnostics.
func (bridge *RuntimeDiagnosticsBridge) Run(ctx context.Context) error {
	backoff := bridge.options.InitialBackoff
	defer bridge.disconnectAfterRun()
	for {
		err := bridge.Refresh(ctx)
		delay := bridge.options.PollInterval
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			delay = backoff
			status := sanitizeRuntimeDiagnosticsStatus(RuntimeDiagnosticsStatus{Connected: false, LastError: err.Error(), NextRetry: delay})
			if publishErr := bridge.sink.PublishRuntimeStatus(ctx, status); publishErr != nil {
				return fmt.Errorf("publish disconnected runtime status: %w", publishErr)
			}
			if backoff < bridge.options.MaxBackoff {
				backoff *= 2
				if backoff > bridge.options.MaxBackoff {
					backoff = bridge.options.MaxBackoff
				}
			}
		} else {
			backoff = bridge.options.InitialBackoff
		}
		if err := bridge.wait(ctx, delay); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

// Disconnect clears every runtime-owned Problems entry and resets generation
// tracking. It is safe to call repeatedly.
func (bridge *RuntimeDiagnosticsBridge) Disconnect(ctx context.Context) error {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()

	update := bridge.state.Clear()
	if err := bridge.sink.ReplaceRuntimeDiagnostics(ctx, update); err != nil {
		return fmt.Errorf("clear runtime diagnostics: %w", err)
	}
	if err := bridge.sink.PublishRuntimeStatus(ctx, RuntimeDiagnosticsStatus{}); err != nil {
		return fmt.Errorf("publish disconnected runtime status: %w", err)
	}
	return nil
}

func (bridge *RuntimeDiagnosticsBridge) disconnectAfterRun() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = bridge.Disconnect(ctx)
}

func runtimeStatusFromSnapshot(snapshot RuntimeDiagnosticsSnapshot, update RuntimeDiagnosticsUpdate) RuntimeDiagnosticsStatus {
	unchecked := append([]string(nil), snapshot.UncheckedRoutes...)
	errorCount := len(update.Unmapped)
	for _, diagnostics := range update.Diagnostics {
		errorCount += len(diagnostics)
	}
	return sanitizeRuntimeDiagnosticsStatus(RuntimeDiagnosticsStatus{
		Connected:       true,
		Generation:      snapshot.Generation,
		ErrorCount:      errorCount,
		CheckedRoutes:   snapshot.CheckedRoutes,
		TotalRoutes:     snapshot.TotalRoutes,
		UncheckedRoutes: unchecked,
		EvictedContexts: snapshot.EvictedContexts,
		UnmappedIssues:  runtimeUnmappedIssueSummaries(update.Unmapped),
	})
}

func waitForRuntimePoll(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
