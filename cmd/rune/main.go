package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/heidaraliy/rune/internal/app"
	v2app "github.com/heidaraliy/rune/internal/app/v2"
	"github.com/heidaraliy/rune/internal/application"
	"github.com/heidaraliy/rune/internal/core"
	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/handoff"
	v2artifacts "github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/markdown"
	v2sqlite "github.com/heidaraliy/rune/internal/storage/sqlite"
	runesync "github.com/heidaraliy/rune/internal/sync"
)

var version = "dev"

type programRunner interface {
	Run() (tea.Model, error)
}

var (
	exitFn          = os.Exit
	writeClipboard  = clipboard.WriteAll
	tmuxSession     = handoff.IsTmuxSession
	writeTmuxBuffer = handoff.LoadTmuxBuffer
	runCodex        = handoff.RunCodex
	newProgram      = func(model tea.Model) programRunner {
		return tea.NewProgram(model, tea.WithAltScreen())
	}
)

func main() {
	if code := run(os.Args[1:], os.Stdout, os.Stderr, os.Stdin, ""); code != 0 {
		exitFn(code)
	}
}

func run(args []string, stdout, stderr io.Writer, stdin io.Reader, cwd string) int {
	if len(args) == 0 {
		return runTUI(stdout, stderr, cwd, false, "")
	}
	switch args[0] {
	case "--version", "-version", "version":
		fmt.Fprintln(stdout, "rune "+displayVersion())
		return 0
	case "--help", "-h", "help":
		printUsage(stdout)
		return 0
	}
	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "add":
		err = runAdd(rest, stdout, stdin, cwd)
	case "list", "ls":
		err = runList(rest, stdout, cwd)
	case "show":
		err = runShow(rest, stdout, cwd)
	case "yank":
		err = runYank(rest, stdout, cwd)
	case "ticket":
		err = runTicket(rest, stdout, cwd)
	case "codex":
		err = runCodexTicket(rest, stdout, stderr, stdin, cwd)
	case "v2":
		err = runV2(rest, stdout, stderr, stdin, cwd)
	case "edit":
		err = runEdit(rest, stdout, stdin, cwd)
	case "done":
		err = runDone(rest, stdout, cwd, true, false)
	case "undone", "open":
		err = runDone(rest, stdout, cwd, false, false)
	case "toggle":
		err = runDone(rest, stdout, cwd, false, true)
	case "tag":
		err = runTag(rest, stdout, cwd, true)
	case "untag":
		err = runTag(rest, stdout, cwd, false)
	case "find", "search":
		err = runFind(rest, stdout, cwd)
	case "projects":
		err = runProjects(rest, stdout, cwd)
	case "tags":
		err = runTags(rest, stdout, cwd)
	case "archive":
		err = runArchive(rest, stdout, cwd)
	case "restore":
		err = runRestore(rest, stdout, cwd)
	case "import":
		err = runImport(rest, stdout, cwd)
	case "init":
		err = runInit(rest, stdout, cwd)
	case "migrate":
		err = runMigrate(rest, stdout, cwd)
	case "path":
		err = runPath(rest, stdout, cwd)
	case "doctor":
		err = runDoctor(rest, stdout, cwd)
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		printError(stderr, err)
		return 1
	}
	return 0
}

func runTUI(stdout, stderr io.Writer, cwd string, global bool, project string) int {
	scope, err := core.ResolveScope(cwd, global, project)
	if err != nil {
		fmt.Fprintf(stderr, "rune: %v\n", err)
		return 1
	}
	model, err := app.New(core.NewStoreForScope(scope), scope)
	if err != nil {
		fmt.Fprintf(stderr, "rune: %v\n", err)
		return 1
	}
	if _, err := newProgram(model).Run(); err != nil {
		fmt.Fprintf(stderr, "rune: %v\n", err)
		return 1
	}
	return 0
}

func runAdd(args []string, stdout io.Writer, stdin io.Reader, cwd string) error {
	fs := flag.NewFlagSet("rune add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tags := fs.String("tag", "", "comma-separated tags")
	project := fs.String("project", "", "project")
	asNote := fs.Bool("note", false, "create a note item")
	body := fs.String("body", "", "body text")
	fromStdin := fs.Bool("stdin", false, "read body from stdin")
	pos, err := parseFlags(fs, args, map[string]bool{"tag": true, "project": true, "body": true})
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New("add requires text")
	}
	if *fromStdin {
		text, err := readAll(stdin)
		if err != nil {
			return err
		}
		*body = text
	}
	scope, store, err := scopedStore(cwd, false, *project)
	if err != nil {
		return err
	}
	item, err := store.Add(scope, core.AddOptions{
		Title:  core.DecodeEscapes(strings.Join(pos, " ")),
		Body:   core.DecodeEscapes(*body),
		Tags:   splitCSV(*tags),
		AsNote: *asNote,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Added %s  %s\n", item.DisplayID, item.Title)
	return nil
}

func runList(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	all := fs.Bool("all", false, "all items")
	done := fs.Bool("done", false, "done items")
	tag := fs.String("tag", "", "tag")
	project := fs.String("project", "", "project")
	sortBy := fs.String("sort", "", "sort by created_at or finished_at")
	reverse := fs.Bool("reverse", false, "reverse sort")
	jsonOut := fs.Bool("json", false, "json")
	pos, err := parseFlags(fs, args, map[string]bool{"tag": true, "project": true, "sort": true})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	items, _, err := store.Items(scope, core.ListOptions{All: *all, Done: *done, Tag: *tag, Sort: *sortBy, Reverse: *reverse, Global: *global, Project: *project})
	if err != nil {
		return err
	}
	if *jsonOut {
		data, err := core.ItemsJSON(items)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printItems(stdout, scope.Home, items)
	return nil
}

func runShow(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune show", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	project := fs.String("project", "", "project")
	raw := fs.Bool("raw", false, "raw")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("show requires one id")
	}
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	item, _, err := store.Resolve(scope, pos[0], *global)
	if err != nil {
		return err
	}
	if *raw {
		fmt.Fprintln(stdout, strings.Join(item.Doc.RawBlock(item), "\n"))
		return nil
	}
	printItemDetail(stdout, scope.Home, item)
	return nil
}

func runYank(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune yank", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	project := fs.String("project", "", "project")
	printTicket := fs.Bool("print", false, "print ticket text")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	_, item, options, text, err := resolveTicket(cwd, *global, *project, pos, "yank")
	if err != nil {
		return err
	}
	if *printTicket {
		fmt.Fprint(stdout, text)
		return nil
	}
	result, err := handoff.YankTicket(text, writeClipboard, tmuxSession(), writeTmuxBuffer)
	if err != nil {
		return fmt.Errorf("yank failed: %w", err)
	}
	fmt.Fprintln(stdout, handoff.YankStatus(item.DisplayID, options.Agent, result))
	return nil
}

func runTicket(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune ticket", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	project := fs.String("project", "", "project")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	_, _, _, text, err := resolveTicket(cwd, *global, *project, pos, "ticket")
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, text)
	return nil
}

func runCodexTicket(args []string, stdout, stderr io.Writer, stdin io.Reader, cwd string) error {
	fs := flag.NewFlagSet("rune codex", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	project := fs.String("project", "", "project")
	reasoning := fs.String("reasoning", "", "Codex reasoning effort")
	minimal := fs.Bool("minimal", false, "use minimal Codex reasoning")
	low := fs.Bool("low", false, "use low Codex reasoning")
	medium := fs.Bool("medium", false, "use medium Codex reasoning")
	high := fs.Bool("high", false, "use high Codex reasoning")
	xhigh := fs.Bool("xhigh", false, "use extra-high Codex reasoning")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "reasoning": true})
	if err != nil {
		return err
	}
	codexOptions, err := codexOptionsFromFlags(*reasoning, []codexReasoningFlag{
		{enabled: *minimal, effort: handoff.CodexReasoningMinimal},
		{enabled: *low, effort: handoff.CodexReasoningLow},
		{enabled: *medium, effort: handoff.CodexReasoningMedium},
		{enabled: *high, effort: handoff.CodexReasoningHigh},
		{enabled: *xhigh, effort: handoff.CodexReasoningXHigh},
	})
	if err != nil {
		return err
	}
	scope, _, _, text, err := resolveTicket(cwd, *global, *project, pos, "codex")
	if err != nil {
		return err
	}
	if err := runCodex(scope.CWD, text, codexOptions, stdin, stdout, stderr); err != nil {
		return fmt.Errorf("codex failed: %w", err)
	}
	return nil
}

func runV2(args []string, stdout, stderr io.Writer, stdin io.Reader, cwd string) error {
	if len(args) == 0 {
		return errors.New("v2 requires a subcommand: init, capture, list, show, edit, delete, restore, status, search, link, links, queue, run, cancel, runs, artifacts, artifact, sync, tui, or import")
	}
	switch args[0] {
	case "init":
		return runV2Init(args[1:], stdout, cwd)
	case "capture", "add":
		return runV2Capture(args[1:], stdout, stdin, cwd)
	case "list":
		return runV2List(args[1:], stdout, cwd)
	case "show":
		return runV2Show(args[1:], stdout, cwd)
	case "edit":
		return runV2Edit(args[1:], stdout, cwd)
	case "delete":
		return runV2Delete(args[1:], stdout, cwd)
	case "restore":
		return runV2Restore(args[1:], stdout, cwd)
	case "status":
		return runV2Status(args[1:], stdout, cwd)
	case "search", "find":
		return runV2Search(args[1:], stdout, cwd)
	case "link":
		return runV2Link(args[1:], stdout, cwd)
	case "links":
		return runV2Links(args[1:], stdout, cwd)
	case "queue":
		return runV2Queue(args[1:], stdout, cwd)
	case "run":
		return runV2Run(args[1:], stdout, cwd)
	case "cancel":
		return runV2Cancel(args[1:], stdout, cwd)
	case "runs":
		return runV2Runs(args[1:], stdout, cwd)
	case "artifacts":
		return runV2Artifacts(args[1:], stdout, cwd)
	case "artifact":
		return runV2Artifact(args[1:], stdout, cwd)
	case "sync":
		return runV2Sync(args[1:], stdout, cwd)
	case "tui":
		return runV2TUI(args[1:], stdout, stderr, cwd)
	case "import":
		return runV2Import(args[1:], stdout, cwd)
	default:
		return fmt.Errorf("unknown v2 subcommand %q", args[0])
	}
}

func runV2Init(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	_, service, closeStore, path, err := openV2Service(cwd, "", *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	_ = service
	fmt.Fprintf(stdout, "Initialized Rune 2 workspace %s at %s\n", *workspace, path)
	return nil
}

func runV2TUI(args []string, stdout, stderr io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	artifactRoot := fs.String("artifact-root", "", "content-addressed artifact directory")
	remote := fs.String("remote", strings.TrimSpace(os.Getenv("RUNE_SYNC_REMOTE")), "file-backed peer directory or sync HTTP URL")
	token := fs.String("token", strings.TrimSpace(os.Getenv("RUNE_SYNC_TOKEN")), "sync HTTP bearer token")
	autoSync := fs.Bool("auto-sync", false, "sync once when the TUI starts (requires --remote)")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true, "artifact-root": true, "remote": true, "token": true})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	if *autoSync && strings.TrimSpace(*remote) == "" {
		return errors.New("v2 tui --auto-sync requires --remote or RUNE_SYNC_REMOTE")
	}
	scope, service, closeService, _, err := openV2ExecutionService(cwd, *project, *workspace, *dbPath, *artifactRoot)
	if err != nil {
		return err
	}
	defer closeService()
	var syncTarget runesync.SyncTarget
	var closeSyncTarget func() error
	if strings.TrimSpace(*remote) != "" {
		syncTarget, closeSyncTarget, err = openV2SyncPeer(*remote, *token)
		if err != nil {
			return err
		}
		defer closeSyncTarget()
	}
	model, err := v2app.NewWithOptions(service, *workspace, scope.Project, v2app.Options{
		SyncTarget: syncTarget,
		AutoSync:   *autoSync,
	})
	if err != nil {
		return err
	}
	if _, err := newProgram(model).Run(); err != nil {
		return err
	}
	_ = stdout
	_ = stderr
	return nil
}

func runV2Capture(args []string, stdout io.Writer, stdin io.Reader, cwd string) error {
	fs := flag.NewFlagSet("rune v2 capture", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	body := fs.String("body", "", "body text")
	parent := fs.String("parent", "", "parent Rune id or rune:// reference")
	order := fs.Int("order", 0, "sibling order; zero appends")
	state := fs.String("state", "", "Rune state: draft, ready, in_progress, complete, blocked, review, or failed")
	facets := fs.String("facets", "", "comma-separated Rune facets")
	var properties repeatedFlag
	var facetProperties repeatedFlag
	fs.Var(&properties, "property", "common Rune property key=value; repeatable")
	fs.Var(&facetProperties, "facet-property", "facet-scoped property facet.key=value; repeatable")
	fromStdin := fs.Bool("stdin", false, "read body from stdin")
	asNote := fs.Bool("note", false, "capture a note instead of a task")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true, "body": true, "parent": true, "order": true, "state": true, "facets": true, "property": true, "facet-property": true})
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New("v2 capture requires text")
	}
	if *fromStdin {
		text, err := readAll(stdin)
		if err != nil {
			return err
		}
		*body = text
	}
	scope, service, closeStore, _, err := openV2Service(cwd, *project, *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	parentID := ""
	if strings.TrimSpace(*parent) != "" {
		parentRune, err := service.Get(context.Background(), *parent)
		if err != nil {
			return fmt.Errorf("resolve parent Rune: %w", err)
		}
		parentID = parentRune.ID
	}
	kind := domain.KindTask
	status := domain.StatusDraft
	if *asNote {
		kind = domain.KindNote
		status = ""
	}
	parsedState, err := domain.NormalizeRuneState(*state)
	if err != nil {
		return err
	}
	var facetSet []domain.RuneFacet
	if strings.TrimSpace(*facets) != "" {
		requestedFacets := make([]domain.RuneFacet, 0, len(splitCSV(*facets)))
		for _, facet := range splitCSV(*facets) {
			requestedFacets = append(requestedFacets, domain.RuneFacet(facet))
		}
		facetSet, err = domain.NormalizeRuneFacets(requestedFacets, kind)
		if err != nil {
			return err
		}
	}
	commonProperties := make(map[string]string)
	for _, assignment := range properties {
		key, value, err := parsePropertyAssignment(assignment)
		if err != nil {
			return err
		}
		commonProperties[key] = value
	}
	facetPropertyValues := make(map[domain.RuneFacet]map[string]string)
	for _, assignment := range facetProperties {
		facet, key, value, err := parseFacetPropertyAssignment(assignment)
		if err != nil {
			return err
		}
		if facetPropertyValues[facet] == nil {
			facetPropertyValues[facet] = make(map[string]string)
		}
		facetPropertyValues[facet][key] = value
	}
	if len(commonProperties) == 0 {
		commonProperties = nil
	}
	if len(facetPropertyValues) == 0 {
		facetPropertyValues = nil
	}
	entity, err := service.Create(context.Background(), domain.Entity{
		Kind:     kind,
		Project:  scope.Project,
		Title:    core.DecodeEscapes(strings.Join(pos, " ")),
		Body:     core.DecodeEscapes(*body),
		ParentID: parentID,
		SiblingOrder: func() int {
			if *order > 0 {
				return *order
			}
			return 0
		}(),
		Status:          status,
		State:           parsedState,
		FacetSet:        facetSet,
		Properties:      commonProperties,
		FacetProperties: facetPropertyValues,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created %s  %s  %s\n", domain.DisplayID(entity.ID), entity.Kind, entity.Title)
	return nil
}

func runV2List(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	kind := fs.String("kind", "", "note or task")
	status := fs.String("status", "", "task status")
	state := fs.String("state", "", "Rune lifecycle state")
	query := fs.String("query", "", "search title and body")
	parent := fs.String("parent", "", "parent Rune id or rune:// reference")
	sortBy := fs.String("sort", "", "updated_at, created_at, title, priority, status, or sibling_order")
	reverse := fs.Bool("reverse", false, "reverse sort direction")
	all := fs.Bool("all", false, "all projects in the workspace")
	global := fs.Bool("global", false, "all projects in the workspace")
	deleted := fs.Bool("deleted", false, "include deleted tombstones")
	jsonOut := fs.Bool("json", false, "json")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true, "kind": true, "status": true, "state": true, "query": true, "parent": true, "sort": true, "deleted": false})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	scope, service, closeStore, _, err := openV2Service(cwd, *project, *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	projectName := scope.Project
	if *all || *global {
		projectName = ""
	}
	options := domain.ListOptions{Project: projectName, Query: *query, IncludeDeleted: *deleted}
	if *kind != "" {
		options.Kind = domain.Kind(*kind)
		if options.Kind != domain.KindNote && options.Kind != domain.KindTask {
			return fmt.Errorf("unknown v2 kind %q; use note or task", *kind)
		}
	}
	if *status != "" {
		parsed, err := domain.NormalizeStatus(*status)
		if err != nil {
			return err
		}
		options.Status = parsed
	}
	if *state != "" {
		parsed, err := domain.NormalizeRuneState(*state)
		if err != nil {
			return err
		}
		options.State = parsed
	}
	if *parent != "" {
		parentRune, err := service.Get(context.Background(), *parent)
		if err != nil {
			return fmt.Errorf("resolve parent Rune: %w", err)
		}
		options.ParentID = parentRune.ID
	}
	if *sortBy != "" {
		parsed, err := domain.NormalizeRuneSort(*sortBy)
		if err != nil {
			return err
		}
		options.SortBy = parsed
	}
	options.Reverse = *reverse
	items, err := service.List(context.Background(), options)
	if err != nil {
		return err
	}
	if *jsonOut {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if len(items) == 0 {
		fmt.Fprintln(stdout, "No v2 entities.")
		return nil
	}
	for _, item := range items {
		status := string(item.State)
		if status == "" {
			status = string(item.Status)
		}
		if status == "" {
			status = "note"
		}
		if item.DeletedAt != nil {
			status = "deleted"
		}
		fmt.Fprintf(stdout, "%s  %-4s  %-10s  %s", domain.DisplayID(item.ID), item.Kind, status, item.Title)
		if item.Project != "" {
			fmt.Fprintf(stdout, "  [%s]", item.Project)
		}
		fmt.Fprintln(stdout)
	}
	return nil
}

func runV2Show(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 show", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	deleted := fs.Bool("deleted", false, "include a deleted tombstone")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true, "deleted": false})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 show requires one id")
	}
	_, service, closeStore, _, err := openV2Service(cwd, "", *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	var entity domain.Entity
	if *deleted {
		entity, err = service.GetIncludingDeleted(context.Background(), pos[0])
	} else {
		entity, err = service.Get(context.Background(), pos[0])
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "ID: %s\nRef: %s\nKind: %s\nState: %s\nTitle: %s\nRevision: %d\n", entity.ID, entity.Reference(), entity.Kind, entity.State, entity.Title, entity.Revision)
	fmt.Fprintf(stdout, "Facets: %s\n", strings.Join(runeFacetNames(entity.Facets()), ", "))
	printRuneProperties(stdout, entity)
	if entity.Project != "" {
		fmt.Fprintf(stdout, "Project: %s\n", entity.Project)
	}
	if entity.Status != "" {
		fmt.Fprintf(stdout, "Status: %s\n", entity.Status)
	}
	if entity.DeletedAt != nil {
		fmt.Fprintf(stdout, "Deleted at: %s\n", entity.DeletedAt.UTC().Format(time.RFC3339))
	}
	if len(entity.Tags) > 0 {
		fmt.Fprintf(stdout, "Tags: #%s\n", strings.Join(entity.Tags, " #"))
	}
	if entity.Body != "" {
		fmt.Fprintf(stdout, "\n%s\n", entity.Body)
	}
	links, err := service.Links(context.Background(), entity.ID)
	if err != nil {
		return err
	}
	if len(links) > 0 {
		fmt.Fprintln(stdout, "\nLinks:")
		for _, link := range links {
			fmt.Fprintf(stdout, "- %s %s %s\n", link.Kind, domain.DisplayID(link.FromID), domain.DisplayID(link.ToID))
		}
	}
	return nil
}

func runV2Edit(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	title := fs.String("title", "", "new title")
	body := fs.String("body", "", "replace body")
	appendBody := fs.String("end", "", "append body")
	parent := fs.String("parent", "", "parent Rune id or rune:// reference")
	order := fs.Int("order", 0, "sibling order")
	state := fs.String("state", "", "Rune lifecycle state")
	facets := fs.String("facets", "", "comma-separated Rune facets")
	var properties repeatedFlag
	var facetProperties repeatedFlag
	var removeProperties repeatedFlag
	var removeFacetProperties repeatedFlag
	fs.Var(&properties, "property", "set common Rune property key=value; repeatable")
	fs.Var(&facetProperties, "facet-property", "set facet property facet.key=value; repeatable")
	fs.Var(&removeProperties, "remove-property", "remove common Rune property key; repeatable")
	fs.Var(&removeFacetProperties, "remove-facet-property", "remove facet property facet.key; repeatable")
	status := fs.String("status", "", "task status")
	priority := fs.Int("priority", 0, "priority")
	revision := fs.Int64("revision", 0, "expected revision")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true, "title": true, "body": true, "end": true, "parent": true, "order": true, "state": true, "facets": true, "property": true, "facet-property": true, "remove-property": true, "remove-facet-property": true, "status": true, "priority": true, "revision": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 edit requires one id")
	}
	provided := make(map[string]bool)
	fs.Visit(func(flag *flag.Flag) {
		provided[flag.Name] = true
	})
	update := domain.Update{ExpectedRevision: *revision}
	changed := false
	parentID := ""
	if provided["title"] {
		update.Title = stringUpdate(core.DecodeEscapes(*title))
		changed = true
	}
	if provided["body"] {
		update.Body = stringUpdate(core.DecodeEscapes(*body))
		changed = true
	}
	if provided["end"] {
		update.AppendBody = stringUpdate(core.DecodeEscapes(*appendBody))
		changed = true
	}
	if provided["parent"] {
		parentID = strings.TrimSpace(*parent)
		update.ParentID = &parentID
		changed = true
	}
	if provided["order"] {
		update.SiblingOrder = order
		changed = true
	}
	if provided["facets"] {
		parsed := splitCSV(*facets)
		update.Facets = &[]domain.RuneFacet{}
		for _, facet := range parsed {
			*update.Facets = append(*update.Facets, domain.RuneFacet(facet))
		}
		changed = true
	}
	if len(properties) > 0 || len(facetProperties) > 0 || len(removeProperties) > 0 || len(removeFacetProperties) > 0 {
		changes, err := propertyChangesFromFlags(properties, facetProperties, removeProperties, removeFacetProperties)
		if err != nil {
			return err
		}
		update.PropertyChanges = changes
		changed = true
	}
	if provided["state"] {
		parsed, err := domain.NormalizeRuneState(*state)
		if err != nil {
			return err
		}
		update.State = &parsed
		changed = true
	}
	if provided["status"] {
		parsed, err := domain.NormalizeStatus(*status)
		if err != nil {
			return err
		}
		update.Status = &parsed
		changed = true
	}
	if provided["priority"] {
		update.Priority = priority
		changed = true
	}
	if !changed {
		return errors.New("v2 edit requires --title, --body, --end, --parent, --order, --facets, --property, --facet-property, --remove-property, --remove-facet-property, --state, --status, or --priority")
	}
	_, service, closeStore, _, err := openV2Service(cwd, "", *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	if provided["parent"] && parentID != "" {
		parentRune, err := service.Get(context.Background(), parentID)
		if err != nil {
			return fmt.Errorf("resolve parent Rune: %w", err)
		}
		parentID = parentRune.ID
		update.ParentID = &parentID
	}
	entity, err := service.Update(context.Background(), pos[0], update)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Updated %s  %s\n", domain.DisplayID(entity.ID), entity.Title)
	return nil
}

func runV2Delete(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	revision := fs.Int64("revision", 0, "expected revision")
	confirm := fs.Bool("confirm", false, "confirm reversible tombstone")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true, "revision": true, "confirm": false})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 delete requires one id")
	}
	if !*confirm {
		return errors.New("v2 delete requires --confirm; this creates a reversible tombstone")
	}
	_, service, closeStore, _, err := openV2Service(cwd, "", *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	entity, err := service.Delete(context.Background(), pos[0], *revision)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Tombstoned %s  %s\n", domain.DisplayID(entity.ID), entity.Title)
	return nil
}

func runV2Restore(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 restore", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	revision := fs.Int64("revision", 0, "expected revision")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true, "revision": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 restore requires one id")
	}
	_, service, closeStore, _, err := openV2Service(cwd, "", *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	entity, err := service.Restore(context.Background(), pos[0], *revision)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Restored %s  %s\n", domain.DisplayID(entity.ID), entity.Title)
	return nil
}

func runV2Status(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	revision := fs.Int64("revision", 0, "expected revision")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true, "revision": true})
	if err != nil {
		return err
	}
	if len(pos) != 2 {
		return errors.New("v2 status requires an id and status")
	}
	status, err := domain.NormalizeStatus(pos[1])
	if err != nil {
		return err
	}
	_, service, closeStore, _, err := openV2Service(cwd, "", *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	entity, err := service.SetTaskStatus(context.Background(), pos[0], status, *revision)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Status %s  %s\n", domain.DisplayID(entity.ID), entity.Status)
	return nil
}

func runV2Sync(args []string, stdout io.Writer, cwd string) error {
	if len(args) > 0 && args[0] == "serve" {
		return runV2SyncServe(args[1:], stdout, cwd)
	}
	fs := flag.NewFlagSet("rune v2 sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	remote := fs.String("remote", strings.TrimSpace(os.Getenv("RUNE_SYNC_REMOTE")), "file-backed peer directory or sync HTTP URL")
	artifactRoot := fs.String("artifact-root", "", "content-addressed artifact directory")
	token := fs.String("token", strings.TrimSpace(os.Getenv("RUNE_SYNC_TOKEN")), "sync HTTP bearer token")
	jsonOut := fs.Bool("json", false, "json")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true, "remote": true, "artifact-root": true, "token": true})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	var status domain.SyncStatus
	var conflicts []domain.Conflict
	var pushed, pulled int
	var protocol string
	if strings.TrimSpace(*remote) != "" {
		_, service, closeStore, _, err := openV2ExecutionService(cwd, "", *workspace, *dbPath, *artifactRoot)
		if err != nil {
			return err
		}
		defer closeStore()
		peer, closePeer, err := openV2SyncPeer(*remote, *token)
		if err != nil {
			return err
		}
		defer closePeer()
		var syncClient application.SyncClient = service
		report, err := syncClient.Sync(context.Background(), peer)
		if err != nil {
			return err
		}
		protocol, status, conflicts, pushed, pulled = report.Protocol, report.Status, report.Conflicts, report.Pushed, report.Pulled
	} else {
		_, service, closeStore, _, err := openV2Service(cwd, "", *workspace, *dbPath)
		if err != nil {
			return err
		}
		defer closeStore()
		status, err = service.SyncStatus(context.Background())
		if err != nil {
			return err
		}
		conflicts, err = service.Conflicts(context.Background())
		if err != nil {
			return err
		}
	}
	if *jsonOut {
		data, err := json.MarshalIndent(struct {
			Protocol  string            `json:"protocol,omitempty"`
			Status    domain.SyncStatus `json:"status"`
			Conflicts []domain.Conflict `json:"conflicts"`
			Pushed    int               `json:"pushed"`
			Pulled    int               `json:"pulled"`
		}{Protocol: protocol, Status: status, Conflicts: conflicts, Pushed: pushed, Pulled: pulled}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if protocol != "" {
		fmt.Fprintf(stdout, "Protocol: %s\n", protocol)
	}
	fmt.Fprintf(stdout, "Workspace: %s\nRemote: %s\nLocal cursor: %d\nPushed changes: %d\nPulled changes: %d\nPending changes: %d\nOpen conflicts: %d\n",
		status.WorkspaceID, status.RemoteState, status.LocalCursor, pushed, pulled, status.PendingChanges, status.OpenConflicts)
	for _, conflict := range conflicts {
		fmt.Fprintf(stdout, "Conflict %s  %s  entity %s  local %d / remote %d\n",
			domain.DisplayID(conflict.ID), conflict.Kind, domain.DisplayID(conflict.EntityID), conflict.LocalRevision, conflict.RemoteRevision)
	}
	return nil
}

func runV2SyncServe(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 sync serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	listen := fs.String("listen", ":8787", "listen address")
	dbPath := fs.String("db", strings.TrimSpace(os.Getenv("RUNE_SYNC_DB")), "server database path")
	workspace := fs.String("workspace", "local", "server workspace id")
	artifactRoot := fs.String("artifact-root", strings.TrimSpace(os.Getenv("RUNE_SYNC_ARTIFACTS")), "server artifact directory")
	token := fs.String("token", strings.TrimSpace(os.Getenv("RUNE_SYNC_TOKEN")), "required sync HTTP bearer token")
	cert := fs.String("cert", "", "TLS certificate path")
	key := fs.String("key", "", "TLS private key path")
	pos, err := parseFlags(fs, args, map[string]bool{"listen": true, "db": true, "workspace": true, "artifact-root": true, "token": true, "cert": true, "key": true})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	if strings.TrimSpace(*token) == "" {
		return errors.New("v2 sync serve requires --token or RUNE_SYNC_TOKEN")
	}
	if (strings.TrimSpace(*cert) == "") != (strings.TrimSpace(*key) == "") {
		return errors.New("v2 sync serve requires both --cert and --key for TLS")
	}
	scope, err := core.ResolveScope(cwd, false, "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(*dbPath) == "" {
		*dbPath = filepath.Join(scope.Home, "rune-sync.db")
	}
	store, err := v2sqlite.Open(*dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if strings.TrimSpace(*artifactRoot) == "" {
		*artifactRoot = filepath.Join(scope.Home, "rune-sync-artifacts")
	}
	artifactStore, err := v2artifacts.Open(*artifactRoot)
	if err != nil {
		return err
	}
	syncServer, err := runesync.NewServer(store, artifactStore, *workspace, *token)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: *listen, Handler: syncServer}
	scheme := "http"
	if strings.TrimSpace(*cert) != "" {
		scheme = "https"
	}
	fmt.Fprintf(stdout, "Rune %s server listening on %s (workspace %s)\n", runesync.ProtocolVersion, syncServerURL(scheme, *listen), *workspace)
	if scheme == "https" {
		err = server.ListenAndServeTLS(*cert, *key)
	} else {
		err = server.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func openV2SyncPeer(remote, token string) (runesync.SyncTarget, func() error, error) {
	remote = strings.TrimSpace(remote)
	if strings.HasPrefix(strings.ToLower(remote), "http://") || strings.HasPrefix(strings.ToLower(remote), "https://") {
		peer, err := runesync.OpenHTTPPeer(remote, token)
		if err != nil {
			return nil, nil, err
		}
		return peer, peer.Close, nil
	}
	peer, err := runesync.OpenFilePeer(remote)
	if err != nil {
		return nil, nil, err
	}
	return peer, peer.Close, nil
}

func syncServerURL(scheme, listen string) string {
	listen = strings.TrimSpace(listen)
	if strings.HasPrefix(listen, ":") {
		listen = "localhost" + listen
	}
	return scheme + "://" + listen
}

func runV2Search(args []string, stdout io.Writer, cwd string) error {
	valueFlags := map[string]bool{"project": true, "db": true, "workspace": true, "kind": true, "status": true, "query": true}
	var flags []string
	var query []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			query = append(query, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if equal := strings.IndexByte(name, '='); equal >= 0 {
			name = name[:equal]
		}
		if valueFlags[name] && !strings.Contains(arg, "=") && index+1 < len(args) {
			index++
			flags = append(flags, args[index])
		}
	}
	if len(query) == 0 {
		return errors.New("v2 search requires a query")
	}
	flags = append(flags, "--query", strings.Join(query, " "))
	return runV2List(flags, stdout, cwd)
}

func runV2Link(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 link", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	kind := fs.String("kind", "references", "link kind")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true, "kind": true})
	if err != nil {
		return err
	}
	if len(pos) != 2 {
		return errors.New("v2 link requires a from id and to id")
	}
	_, service, closeStore, _, err := openV2Service(cwd, "", *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	from, err := service.Get(context.Background(), pos[0])
	if err != nil {
		return err
	}
	to, err := service.Get(context.Background(), pos[1])
	if err != nil {
		return err
	}
	link, err := service.Link(context.Background(), from.ID, to.ID, *kind)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Linked %s %s %s\n", domain.DisplayID(from.ID), link.Kind, domain.DisplayID(to.ID))
	return nil
}

func runV2Links(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 links", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	pos, err := parseFlags(fs, args, map[string]bool{"db": true, "workspace": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 links requires one id")
	}
	_, service, closeStore, _, err := openV2Service(cwd, "", *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	entity, err := service.Get(context.Background(), pos[0])
	if err != nil {
		return err
	}
	links, err := service.Links(context.Background(), entity.ID)
	if err != nil {
		return err
	}
	for _, link := range links {
		fmt.Fprintf(stdout, "%s  %s  %s\n", link.Kind, domain.DisplayID(link.FromID), domain.DisplayID(link.ToID))
	}
	return nil
}

func runV2Queue(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 queue", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	provider := fs.String("provider", "fake", "agent provider")
	model := fs.String("model", "local", "model profile")
	permission := fs.String("permission", "read-only", "permission policy")
	artifactRoot := fs.String("artifact-root", "", "content-addressed artifact directory")
	jsonOut := fs.Bool("json", false, "json")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true, "provider": true, "model": true, "permission": true, "artifact-root": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 queue requires one task id")
	}
	policy, err := domain.NormalizePermissionPolicy(*permission)
	if err != nil {
		return err
	}
	_, service, closeService, _, err := openV2ExecutionService(cwd, *project, *workspace, *dbPath, *artifactRoot)
	if err != nil {
		return err
	}
	defer closeService()
	run, err := service.QueueRun(context.Background(), pos[0], *provider, *model, policy)
	if err != nil {
		return err
	}
	if *jsonOut {
		data, err := json.MarshalIndent(run, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintf(stdout, "Queued run %s  task %s  provider %s\n", domain.DisplayID(run.ID), domain.DisplayID(run.TaskID), run.Provider)
	fmt.Fprintf(stdout, "Context artifact: %s\n", domain.DisplayID(run.ContextArtifactID))
	return nil
}

func runV2Run(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	artifactRoot := fs.String("artifact-root", "", "content-addressed artifact directory")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true, "artifact-root": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 run requires one queued run id")
	}
	_, service, closeService, _, err := openV2ExecutionService(cwd, *project, *workspace, *dbPath, *artifactRoot)
	if err != nil {
		return err
	}
	defer closeService()
	run, err := service.ExecuteRun(context.Background(), pos[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Run %s  %s  %s\n", domain.DisplayID(run.ID), run.Status, run.Summary)
	return nil
}

func runV2Cancel(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 cancel", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 cancel requires one run id")
	}
	_, service, closeService, _, err := openV2Service(cwd, *project, *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeService()
	run, err := service.CancelRun(context.Background(), pos[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Run %s  %s\n", domain.DisplayID(run.ID), run.Status)
	return nil
}

func runV2Runs(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 runs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	status := fs.String("status", "", "run status")
	jsonOut := fs.Bool("json", false, "json")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true, "status": true})
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return errors.New("v2 runs accepts at most one task id")
	}
	_, service, closeService, _, err := openV2Service(cwd, *project, *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeService()
	options := domain.RunListOptions{}
	if *status != "" {
		options.Status, err = domain.NormalizeRunStatus(*status)
		if err != nil {
			return err
		}
	}
	if len(pos) == 1 {
		task, err := service.Get(context.Background(), pos[0])
		if err != nil {
			return err
		}
		options.TaskID = task.ID
	}
	runs, err := service.Runs(context.Background(), options)
	if err != nil {
		return err
	}
	if *jsonOut {
		data, err := json.MarshalIndent(runs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if len(runs) == 0 {
		fmt.Fprintln(stdout, "No v2 runs.")
		return nil
	}
	for _, run := range runs {
		fmt.Fprintf(stdout, "%s  %-9s  task %s  %s", domain.DisplayID(run.ID), run.Status, domain.DisplayID(run.TaskID), run.Provider)
		if run.Summary != "" {
			fmt.Fprintf(stdout, "  %s", run.Summary)
		}
		fmt.Fprintln(stdout)
	}
	return nil
}

func runV2Artifacts(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 artifacts", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	jsonOut := fs.Bool("json", false, "json")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 artifacts requires one run id")
	}
	_, service, closeService, _, err := openV2Service(cwd, *project, *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeService()
	items, err := service.Artifacts(context.Background(), pos[0])
	if err != nil {
		return err
	}
	if *jsonOut {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if len(items) == 0 {
		fmt.Fprintln(stdout, "No v2 artifacts.")
		return nil
	}
	for _, artifact := range items {
		fmt.Fprintf(stdout, "%s  %-8s  %-20s  %d bytes  sha256:%s\n", domain.DisplayID(artifact.ID), artifact.Kind, artifact.Name, artifact.SizeBytes, artifact.SHA256[:12])
	}
	return nil
}

func runV2Artifact(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 artifact", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	artifactRoot := fs.String("artifact-root", "", "content-addressed artifact directory")
	raw := fs.Bool("raw", false, "write content only")
	jsonOut := fs.Bool("json", false, "json metadata")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true, "artifact-root": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 artifact requires one artifact id")
	}
	_, service, closeService, _, err := openV2ExecutionService(cwd, *project, *workspace, *dbPath, *artifactRoot)
	if err != nil {
		return err
	}
	defer closeService()
	if *jsonOut {
		artifact, err := service.Artifact(context.Background(), pos[0])
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(artifact, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	artifact, content, err := service.ReadArtifact(context.Background(), pos[0])
	if err != nil {
		return err
	}
	if *raw {
		_, err = stdout.Write(content)
		return err
	}
	fmt.Fprintf(stdout, "ID: %s\nKind: %s\nName: %s\nMedia type: %s\nSize: %d bytes\nSHA-256: %s\nRetention: %s\nSecret state: %s\n\nContent:\n", artifact.ID, artifact.Kind, artifact.Name, artifact.MediaType, artifact.SizeBytes, artifact.SHA256, artifact.Retention, artifact.SecretState)
	if strings.HasPrefix(artifact.MediaType, "text/") || artifact.MediaType == "application/json" {
		fmt.Fprintln(stdout, string(content))
	} else {
		fmt.Fprintln(stdout, "(binary artifact; use --raw to write content)")
	}
	return nil
}

func runV2Import(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune v2 import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	dbPath := fs.String("db", "", "v2 database path")
	workspace := fs.String("workspace", "local", "workspace id")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "db": true, "workspace": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("v2 import requires one Markdown file")
	}
	bundle, err := markdown.ReadFile(pos[0], *project, time.Now().UTC())
	if err != nil {
		return err
	}
	_, service, closeStore, _, err := openV2Service(cwd, bundle.Project, *workspace, *dbPath)
	if err != nil {
		return err
	}
	defer closeStore()
	report, err := service.Import(context.Background(), bundle)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Imported %d item(s), skipped %d from %s\n", report.Created, report.Skipped, report.SourcePath)
	for _, warning := range report.Warnings {
		fmt.Fprintf(stdout, "Warning: %s\n", warning)
	}
	return nil
}

func openV2Service(cwd, project, workspace, dbPath string) (core.Scope, application.V2Service, func() error, string, error) {
	scope, store, closeStore, dbPath, err := openV2Store(cwd, project, workspace, dbPath)
	if err != nil {
		return core.Scope{}, application.V2Service{}, nil, "", err
	}
	return scope, application.NewV2Service(store, workspace), closeStore, dbPath, nil
}

func openV2ExecutionService(cwd, project, workspace, dbPath, artifactRoot string) (core.Scope, application.V2Service, func() error, string, error) {
	scope, store, closeStore, dbPath, err := openV2Store(cwd, project, workspace, dbPath)
	if err != nil {
		return core.Scope{}, application.V2Service{}, nil, "", err
	}
	if strings.TrimSpace(artifactRoot) == "" {
		artifactRoot = strings.TrimSpace(os.Getenv("RUNE_V2_ARTIFACTS"))
	}
	if strings.TrimSpace(artifactRoot) == "" {
		artifactRoot = filepath.Join(scope.Home, "rune-v2-artifacts")
	}
	artifactStore, err := v2artifacts.Open(artifactRoot)
	if err != nil {
		_ = closeStore()
		return core.Scope{}, application.V2Service{}, nil, "", err
	}
	return scope, application.NewV2ExecutionService(store, workspace, artifactStore), closeStore, dbPath, nil
}

func openV2Store(cwd, project, workspace, dbPath string) (core.Scope, *v2sqlite.Store, func() error, string, error) {
	scope, err := core.ResolveScope(cwd, false, project)
	if err != nil {
		return core.Scope{}, nil, nil, "", err
	}
	if strings.TrimSpace(dbPath) == "" {
		dbPath = strings.TrimSpace(os.Getenv("RUNE_V2_DB"))
	}
	if strings.TrimSpace(dbPath) == "" {
		dbPath = v2sqlite.DefaultPath(scope.Home)
	}
	store, err := v2sqlite.Open(dbPath)
	if err != nil {
		return core.Scope{}, nil, nil, "", err
	}
	return scope, store, store.Close, dbPath, nil
}

type codexReasoningFlag struct {
	enabled bool
	effort  handoff.CodexReasoningEffort
}

func codexOptionsFromFlags(reasoning string, flags []codexReasoningFlag) (handoff.CodexOptions, error) {
	var selected []handoff.CodexReasoningEffort
	if strings.TrimSpace(reasoning) != "" {
		effort, err := handoff.ParseCodexReasoningEffort(reasoning)
		if err != nil {
			return handoff.CodexOptions{}, err
		}
		selected = append(selected, effort)
	}
	for _, flag := range flags {
		if flag.enabled {
			selected = append(selected, flag.effort)
		}
	}
	if len(selected) > 1 {
		return handoff.CodexOptions{}, errors.New("choose only one Codex reasoning effort")
	}
	if len(selected) == 0 {
		return handoff.CodexOptions{}, nil
	}
	return handoff.CodexOptions{ReasoningEffort: selected[0]}, nil
}

func resolveTicket(cwd string, global bool, project string, pos []string, command string) (core.Scope, *core.Item, core.YankOptions, string, error) {
	if len(pos) != 1 {
		return core.Scope{}, nil, core.YankOptions{}, "", fmt.Errorf("%s requires one id", command)
	}
	scope, store, err := scopedStore(cwd, global, project)
	if err != nil {
		return core.Scope{}, nil, core.YankOptions{}, "", err
	}
	item, _, err := store.Resolve(scope, pos[0], global)
	if err != nil {
		return core.Scope{}, nil, core.YankOptions{}, "", err
	}
	options := core.YankOptionsForItem(item)
	return scope, item, options, core.YankTicketTextWithOptions(item, scope.Home, options), nil
}

func runEdit(args []string, stdout io.Writer, stdin io.Reader, cwd string) error {
	fs := flag.NewFlagSet("rune edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	project := fs.String("project", "", "project")
	appendText := fs.String("end", "", "append text")
	replaceText := fs.String("replace", "", "replace body")
	title := fs.String("title", "", "new title")
	fromStdin := fs.Bool("stdin", false, "read stdin")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "end": true, "replace": true, "title": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("edit requires one id")
	}
	if *fromStdin {
		text, err := readAll(stdin)
		if err != nil {
			return err
		}
		if *replaceText != "" {
			*replaceText = text
		} else {
			*appendText = text
		}
	}
	if *appendText == "" && *replaceText == "" && *title == "" {
		return errors.New("edit requires --end, --replace, --title, or --stdin")
	}
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	item, err := store.Edit(scope, pos[0], core.EditOptions{
		Title:       core.DecodeEscapes(*title),
		Append:      core.DecodeEscapes(*appendText),
		ReplaceBody: core.DecodeEscapes(*replaceText),
	}, *global)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Updated %s  %s\n", item.DisplayID, item.Title)
	return nil
}

func runDone(args []string, stdout io.Writer, cwd string, done bool, toggle bool) error {
	fs := flag.NewFlagSet("rune done", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	project := fs.String("project", "", "project")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("task command requires one id")
	}
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	item, err := store.SetDone(scope, pos[0], done, toggle, *global)
	if err != nil {
		return err
	}
	state := "Opened"
	if item.Done {
		state = "Done"
	}
	fmt.Fprintf(stdout, "%s %s  %s\n", state, item.DisplayID, item.Title)
	return nil
}

func runTag(args []string, stdout io.Writer, cwd string, add bool) error {
	fs := flag.NewFlagSet("rune tag", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	project := fs.String("project", "", "project")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	if len(pos) != 2 {
		return errors.New("tag commands require an id and comma-separated tags")
	}
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	opts := core.EditOptions{}
	if add {
		opts.Tags = splitCSV(pos[1])
	} else {
		opts.Untags = splitCSV(pos[1])
	}
	item, err := store.Edit(scope, pos[0], opts, *global)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Tagged %s  %s\n", item.DisplayID, strings.Join(item.Tags, ","))
	return nil
}

func runFind(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune find", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	global := fs.Bool("global", false, "all projects")
	project := fs.String("project", "", "project")
	tag := fs.String("tag", "", "tag")
	sortBy := fs.String("sort", "", "sort by created_at or finished_at")
	reverse := fs.Bool("reverse", false, "reverse sort")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "tag": true, "sort": true})
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New("find requires a query")
	}
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	items, _, err := store.Items(scope, core.ListOptions{All: true, Query: strings.Join(pos, " "), Tag: *tag, Sort: *sortBy, Reverse: *reverse, Global: *global, Project: *project})
	if err != nil {
		return err
	}
	printItems(stdout, scope.Home, items)
	return nil
}

func runProjects(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune projects", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pos, err := parseFlags(fs, args, nil)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	scope, store, err := scopedStore(cwd, false, "")
	if err != nil {
		return err
	}
	_ = scope
	projects, err := store.ProjectNames()
	if err != nil {
		return err
	}
	for _, project := range projects {
		fmt.Fprintln(stdout, project)
	}
	return nil
}

func runTags(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune tags", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pos, err := parseFlags(fs, args, nil)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	_, store, err := scopedStore(cwd, true, "")
	if err != nil {
		return err
	}
	counts, err := store.TagCounts()
	if err != nil {
		return err
	}
	var tags []string
	for tag := range counts {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		fmt.Fprintf(stdout, "%s %d\n", tag, counts[tag])
	}
	return nil
}

func runArchive(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune archive", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	done := fs.Bool("done", false, "archive done items")
	project := fs.String("project", "", "project")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	if !*done {
		return errors.New("archive currently requires --done")
	}
	scope, store, err := scopedStore(cwd, false, *project)
	if err != nil {
		return err
	}
	count, path, err := store.ArchiveDone(scope)
	if err != nil {
		return err
	}
	if count == 0 {
		fmt.Fprintln(stdout, "No completed items to archive.")
		return nil
	}
	fmt.Fprintf(stdout, "Archived %d item(s) to %s\n", count, path)
	return nil
}

func runRestore(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune restore", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	scope, store, err := scopedStore(cwd, false, *project)
	if err != nil {
		return err
	}
	count, paths, err := store.RestoreArchivedProject(scope)
	if err != nil {
		return err
	}
	if count == 0 {
		fmt.Fprintln(stdout, "No archived project items to restore.")
		return nil
	}
	target := core.ProjectPath(scope.Home, scope.Project)
	if scope.Layout == core.StoreLayoutFiles {
		target = core.ProjectDir(scope.Home, scope.Project)
	}
	fmt.Fprintf(stdout, "Restored %d item(s) into %s\n", count, target)
	for _, path := range paths {
		fmt.Fprintf(stdout, "Updated archive: %s\n", path)
	}
	return nil
}

func runImport(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("import requires a markdown file")
	}
	scope, store, err := scopedStore(cwd, false, *project)
	if err != nil {
		return err
	}
	proj := *project
	if proj == "" {
		proj = scope.Project
	}
	count, path, err := store.Import(pos[0], proj)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Imported %d item id(s) into %s\n", count, path)
	return nil
}

func runInit(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	config, home, err := core.InitLocalStore(cwd, *project)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Initialized %s for project %s\n", home, config.Project)
	return nil
}

func runMigrate(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune migrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project")
	source := fs.String("source", "", "source markdown file")
	force := fs.Bool("force", false, "merge into existing note files")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "source": true})
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return errors.New("migrate accepts at most one source file")
	}
	if len(pos) == 1 {
		*source = pos[0]
	}
	report, config, err := core.MigrateProjectToLocal(cwd, *project, *source, *force, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Migrated %d item(s) into %d file(s) for project %s\n", report.Items, report.Files, config.Project)
	fmt.Fprintf(stdout, "Source left unchanged: %s\n", report.Source)
	fmt.Fprintf(stdout, "Target: %s\n", report.Target)
	if report.AssignedIDs > 0 {
		fmt.Fprintf(stdout, "Assigned %d missing id(s)\n", report.AssignedIDs)
	}
	return nil
}

func runPath(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune path", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	storePath := fs.Bool("store", false, "store path")
	global := fs.Bool("global", false, "global")
	project := fs.String("project", "", "project")
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	if *storePath {
		fmt.Fprintln(stdout, scope.Home)
		return nil
	}
	if len(pos) == 1 {
		item, _, err := store.Resolve(scope, pos[0], *global)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s:%d\n", item.Source, item.Line+1)
		return nil
	}
	if len(pos) > 1 {
		return errors.New("path accepts at most one id")
	}
	docs, err := store.LoadScope(scope)
	if err != nil {
		return err
	}
	if len(docs) > 0 {
		fmt.Fprintln(stdout, docs[0].Path)
	} else if scope.Layout == core.StoreLayoutFiles && scope.Project != "" {
		fmt.Fprintln(stdout, core.ProjectDir(scope.Home, scope.Project))
	}
	return nil
}

func runDoctor(args []string, stdout io.Writer, cwd string) error {
	fs := flag.NewFlagSet("rune doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fix := fs.Bool("fix", false, "fix")
	pos, err := parseFlags(fs, args, nil)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	_, store, err := scopedStore(cwd, true, "")
	if err != nil {
		return err
	}
	report, err := store.Doctor(*fix)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Missing IDs: %d\n", report.MissingIDs)
	fmt.Fprintf(stdout, "Duplicate IDs: %d\n", len(report.DuplicateIDs))
	if *fix {
		fmt.Fprintf(stdout, "Fixed: %d\n", report.Fixed)
	}
	return nil
}

func scopedStore(cwd string, global bool, project string) (core.Scope, core.Store, error) {
	scope, err := core.ResolveScope(cwd, global, project)
	if err != nil {
		return core.Scope{}, core.Store{}, err
	}
	store := core.NewStoreForScope(scope)
	return scope, store, nil
}

func parseFlags(fs *flag.FlagSet, args []string, valueFlags map[string]bool) ([]string, error) {
	var flagsPart []string
	var pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pos = append(pos, arg)
			continue
		}
		flagsPart = append(flagsPart, arg)
		name := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if valueFlags[name] && !strings.Contains(arg, "=") {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs an argument: --%s", name)
			}
			i++
			flagsPart = append(flagsPart, args[i])
		}
	}
	if err := fs.Parse(flagsPart); err != nil {
		return nil, err
	}
	return pos, nil
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func parsePropertyAssignment(value string) (string, string, error) {
	separator := strings.IndexByte(value, '=')
	if separator < 1 {
		return "", "", fmt.Errorf("property %q must use key=value", value)
	}
	key := strings.TrimSpace(value[:separator])
	if key == "" {
		return "", "", errors.New("Rune property name is required")
	}
	return key, value[separator+1:], nil
}

func parseFacetPropertyAssignment(value string) (domain.RuneFacet, string, string, error) {
	assignmentKey, propertyValue, err := parsePropertyAssignment(value)
	if err != nil {
		return "", "", "", err
	}
	separator := strings.IndexByte(assignmentKey, '.')
	if separator < 1 || separator == len(assignmentKey)-1 {
		return "", "", "", fmt.Errorf("facet property %q must use facet.key=value", value)
	}
	facet, err := domain.NormalizeRuneFacet(assignmentKey[:separator])
	if err != nil {
		return "", "", "", err
	}
	key := strings.TrimSpace(assignmentKey[separator+1:])
	if key == "" {
		return "", "", "", errors.New("Rune property name is required")
	}
	return facet, key, propertyValue, nil
}

func parseFacetPropertyKey(value string) (domain.RuneFacet, string, error) {
	key := strings.TrimSpace(value)
	separator := strings.IndexByte(key, '.')
	if separator < 1 || separator == len(key)-1 {
		return "", "", fmt.Errorf("facet property %q must use facet.key", value)
	}
	facet, err := domain.NormalizeRuneFacet(key[:separator])
	if err != nil {
		return "", "", err
	}
	propertyKey := strings.TrimSpace(key[separator+1:])
	if propertyKey == "" {
		return "", "", errors.New("Rune property name is required")
	}
	return facet, propertyKey, nil
}

func propertyChangesFromFlags(properties, facetProperties, removals, facetRemovals []string) ([]domain.RunePropertyChange, error) {
	changes := make([]domain.RunePropertyChange, 0, len(properties)+len(facetProperties)+len(removals)+len(facetRemovals))
	for _, assignment := range properties {
		key, value, err := parsePropertyAssignment(assignment)
		if err != nil {
			return nil, err
		}
		changes = append(changes, domain.RunePropertyChange{Key: key, Value: value})
	}
	for _, assignment := range facetProperties {
		facet, key, value, err := parseFacetPropertyAssignment(assignment)
		if err != nil {
			return nil, err
		}
		changes = append(changes, domain.RunePropertyChange{Facet: facet, Key: key, Value: value})
	}
	for _, key := range removals {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, errors.New("Rune property name is required")
		}
		changes = append(changes, domain.RunePropertyChange{Key: key, Delete: true})
	}
	for _, key := range facetRemovals {
		facet, propertyKey, err := parseFacetPropertyKey(key)
		if err != nil {
			return nil, err
		}
		changes = append(changes, domain.RunePropertyChange{Facet: facet, Key: propertyKey, Delete: true})
	}
	return changes, nil
}

func readAll(r io.Reader) (string, error) {
	if r == nil {
		return "", nil
	}
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	return buf.String(), err
}

func stringUpdate(value string) *string {
	return &value
}

func runeFacetNames(facets []domain.RuneFacet) []string {
	names := make([]string, len(facets))
	for index, facet := range facets {
		names[index] = string(facet)
	}
	return names
}

func printRuneProperties(w io.Writer, entity domain.Entity) {
	propertyKeys := make([]string, 0, len(entity.Properties))
	for key := range entity.Properties {
		propertyKeys = append(propertyKeys, key)
	}
	sort.Strings(propertyKeys)
	for _, key := range propertyKeys {
		fmt.Fprintf(w, "Property: %s=%s\n", key, entity.Properties[key])
	}
	facetNames := make([]string, 0, len(entity.FacetProperties))
	for facet := range entity.FacetProperties {
		facetNames = append(facetNames, string(facet))
	}
	sort.Strings(facetNames)
	for _, facetName := range facetNames {
		values := entity.FacetProperties[domain.RuneFacet(facetName)]
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(w, "Facet property: %s.%s=%s\n", facetName, key, values[key])
		}
	}
}

func printItems(w io.Writer, home string, items []*core.Item) {
	if len(items) == 0 {
		fmt.Fprintln(w, "No items.")
		return
	}

	styles := newCLIStyles(w)
	idWidth := 0
	for _, item := range items {
		idWidth = max(idWidth, lipgloss.Width(item.DisplayID))
	}

	fmt.Fprintln(w, styles.header.Render(itemCountLabel(len(items))))
	fmt.Fprintln(w)
	for idx, item := range items {
		if idx > 0 {
			fmt.Fprintln(w)
		}
		printItemCard(w, home, styles, idWidth, item)
	}
}

func printItemCard(w io.Writer, home string, styles cliStyles, idWidth int, item *core.Item) {
	itemText := wrapText(itemDisplayText(item), listCardBodyWidth)
	if len(itemText) == 0 {
		itemText = []string{""}
	}
	indent := strings.Repeat(" ", idWidth+2)
	itemStyle := itemStyle(styles, item)

	fmt.Fprintf(w, "%s  %s\n", styles.id.Render(padRight(item.DisplayID, idWidth)), itemStyle.Render(itemText[0]))
	for _, line := range itemText[1:] {
		fmt.Fprintf(w, "%s%s\n", indent, itemStyle.Render(line))
	}

	source := listSourceLabel(home, item)
	tags := tagDisplayText(item.Tags)
	if source != "" || tags != "" {
		fmt.Fprint(w, indent)
		if source != "" {
			fmt.Fprint(w, styles.source.Render(source))
		}
		if tags != "" {
			if source != "" {
				fmt.Fprint(w, "  ")
			}
			fmt.Fprint(w, styles.tag.Render(tags))
		}
		fmt.Fprintln(w)
	}
}

const listCardBodyWidth = 72

func wrapText(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	current := ""
	for _, word := range strings.Fields(text) {
		for lipgloss.Width(word) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			var head string
			head, word = splitAtWidth(word, width)
			lines = append(lines, head)
		}
		if current == "" {
			current = word
			continue
		}
		next := current + " " + word
		if lipgloss.Width(next) <= width {
			current = next
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitAtWidth(text string, width int) (string, string) {
	if width <= 0 {
		return "", text
	}
	runes := []rune(text)
	cut := 0
	for cut < len(runes) && lipgloss.Width(string(runes[:cut+1])) <= width {
		cut++
	}
	if cut == 0 {
		cut = 1
	}
	return string(runes[:cut]), string(runes[cut:])
}

func printItemDetail(w io.Writer, home string, item *core.Item) {
	styles := newCLIStyles(w)
	fmt.Fprintf(w, "%s  %s\n", styles.id.Render(item.DisplayID), itemStyle(styles, item).Render(itemDisplayText(item)))
	printDetailMeta(w, styles, "status", core.ItemStatus(item))
	if item.Heading != "" {
		printDetailMeta(w, styles, "heading", item.Heading)
	}
	if len(item.Tags) > 0 {
		printDetailMeta(w, styles, "tags", tagDisplayText(item.Tags))
	}
	if item.Source != "" {
		printDetailMeta(w, styles, "source", fmt.Sprintf("%s:%d", core.SourceLabel(item, home), item.Line+1))
	}
	body := item.Body()
	if body != "" {
		fmt.Fprintln(w)
		scanner := bufio.NewScanner(strings.NewReader(body))
		for scanner.Scan() {
			fmt.Fprintln(w, scanner.Text())
		}
	}
}

type cliStyles struct {
	header lipgloss.Style
	id     lipgloss.Style
	label  lipgloss.Style
	open   lipgloss.Style
	done   lipgloss.Style
	note   lipgloss.Style
	tag    lipgloss.Style
	source lipgloss.Style
	meta   lipgloss.Style
}

func newCLIStyles(w io.Writer) cliStyles {
	renderer := lipgloss.NewRenderer(w)
	return cliStyles{
		header: renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		id:     renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("183")),
		label:  renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("111")),
		open:   renderer.NewStyle().Foreground(lipgloss.Color("222")),
		done:   renderer.NewStyle().Foreground(lipgloss.Color("108")),
		note:   renderer.NewStyle().Foreground(lipgloss.Color("159")),
		tag:    renderer.NewStyle().Foreground(lipgloss.Color("111")),
		source: renderer.NewStyle().Foreground(lipgloss.Color("245")),
		meta:   renderer.NewStyle().Foreground(lipgloss.Color("252")),
	}
}

func itemCountLabel(count int) string {
	if count == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", count)
}

func itemDisplayText(item *core.Item) string {
	if item == nil {
		return ""
	}
	if item.Type != core.ItemTask {
		return "note " + item.Title
	}
	if item.Done {
		return "[x] " + item.Title
	}
	return "[ ] " + item.Title
}

func itemStyle(styles cliStyles, item *core.Item) lipgloss.Style {
	if item == nil || item.Type != core.ItemTask {
		return styles.note
	}
	if item.Done {
		return styles.done
	}
	return styles.open
}

func tagDisplayText(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return "#" + strings.Join(tags, " #")
}

func listSourceLabel(home string, item *core.Item) string {
	if item == nil {
		return ""
	}
	if item.Doc != nil && item.Doc.Kind == "project" && item.Project != "" {
		return item.Project
	}
	if item.Source != "" {
		return core.SourceLabel(item, home)
	}
	if item.Doc != nil {
		return item.Doc.RelPath(home)
	}
	return item.Project
}

func printDetailMeta(w io.Writer, styles cliStyles, label, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(w, "%s  %s\n", styles.label.Render(padRight(label, 7)), styles.meta.Render(value))
}

func padRight(value string, width int) string {
	if width <= 0 {
		return value
	}
	missing := width - lipgloss.Width(value)
	if missing <= 0 {
		return value
	}
	return value + strings.Repeat(" ", missing)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func printError(w io.Writer, err error) {
	var ambiguous core.AmbiguousIDError
	if errors.As(err, &ambiguous) {
		fmt.Fprintf(w, "rune: %s\n\n", ambiguous.Error())
		for _, item := range ambiguous.Matches {
			fmt.Fprintf(w, "%-4s %s\n", item.DisplayID, item.Title)
		}
		fmt.Fprintln(w, "\nUse a longer id.")
		return
	}
	var ambiguousV2 *v2sqlite.AmbiguousIDError
	if errors.As(err, &ambiguousV2) {
		fmt.Fprintf(w, "rune: %s\n\n", ambiguousV2.Error())
		for _, item := range ambiguousV2.Matches {
			fmt.Fprintf(w, "%-8s %s\n", domain.DisplayID(item.ID), item.Title)
		}
		fmt.Fprintln(w, "\nUse a longer id.")
		return
	}
	var ambiguousRun *v2sqlite.AmbiguousRunIDError
	if errors.As(err, &ambiguousRun) {
		fmt.Fprintf(w, "rune: %s\n\n", ambiguousRun.Error())
		for _, run := range ambiguousRun.Matches {
			fmt.Fprintf(w, "%-8s %s %s\n", domain.DisplayID(run.ID), run.Status, domain.DisplayID(run.TaskID))
		}
		fmt.Fprintln(w, "\nUse a longer id.")
		return
	}
	var ambiguousArtifact *v2sqlite.AmbiguousArtifactIDError
	if errors.As(err, &ambiguousArtifact) {
		fmt.Fprintf(w, "rune: %s\n\n", ambiguousArtifact.Error())
		for _, artifact := range ambiguousArtifact.Matches {
			fmt.Fprintf(w, "%-8s %s\n", domain.DisplayID(artifact.ID), artifact.Name)
		}
		fmt.Fprintln(w, "\nUse a longer id.")
		return
	}
	fmt.Fprintf(w, "rune: %v\n", err)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Rune captures project memory from the terminal.

Usage:
  rune
  rune add "fix stuns" --tag combat,bug
  rune list [--global] [--all] [--done] [--tag tag] [--sort created_at|finished_at] [--reverse]
  rune yank <id> [--print]
  rune ticket <id>
  rune codex <id> [--minimal|--low|--medium|--high|--xhigh]
	  rune v2 <init|capture|list|show|status|search|link|links|queue|run|cancel|runs|artifacts|artifact|sync [serve]|tui|import> ...
  rune edit <id> --end "details with \n newlines"
  rune done <id>
  rune find "query" --global
  rune init [--project p]
  rune migrate [file] [--project p] [--force]

Commands:
  add, list, show, yank, ticket, codex, v2, edit, done, undone, toggle, tag, untag, find
  projects, tags, archive, restore, import, init, migrate, path, doctor`)
}

func displayVersion() string {
	display := version
	if display == "" || display == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			display = info.Main.Version
		}
	}
	return strings.TrimPrefix(display, "v")
}
