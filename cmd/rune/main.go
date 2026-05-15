package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/rune/internal/app"
	"github.com/heidaraliy/rune/internal/core"
)

var version = "dev"

type programRunner interface {
	Run() (tea.Model, error)
}

var (
	exitFn         = os.Exit
	writeClipboard = clipboard.WriteAll
	newProgram     = func(model app.Model) programRunner {
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
	case "today":
		err = runQuickNote(rest, stdout, stdin, cwd, "today")
	case "inbox":
		err = runQuickNote(rest, stdout, stdin, cwd, "inbox")
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
	model, err := app.New(core.NewStore(scope.Home), scope)
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
	global := fs.Bool("global", false, "add to global inbox")
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
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	if *global {
		scope.Inbox = true
		scope.Global = false
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
	jsonOut := fs.Bool("json", false, "json")
	pos, err := parseFlags(fs, args, map[string]bool{"tag": true, "project": true})
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
	items, _, err := store.Items(scope, core.ListOptions{All: *all, Done: *done, Tag: *tag, Global: *global, Project: *project})
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
	pos, err := parseFlags(fs, args, map[string]bool{"project": true})
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("yank requires one id")
	}
	scope, store, err := scopedStore(cwd, *global, *project)
	if err != nil {
		return err
	}
	item, _, err := store.Resolve(scope, pos[0], *global)
	if err != nil {
		return err
	}
	if err := writeClipboard(core.YankTicketText(item, scope.Home)); err != nil {
		return fmt.Errorf("yank failed: %w", err)
	}
	fmt.Fprintf(stdout, "Yanked %s for %s.\n", item.DisplayID, core.YankAgent)
	return nil
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
	pos, err := parseFlags(fs, args, map[string]bool{"project": true, "tag": true})
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
	items, _, err := store.Items(scope, core.ListOptions{All: true, Query: strings.Join(pos, " "), Tag: *tag, Global: *global, Project: *project})
	if err != nil {
		return err
	}
	printItems(stdout, scope.Home, items)
	return nil
}

func runQuickNote(args []string, stdout io.Writer, stdin io.Reader, cwd, kind string) error {
	fs := flag.NewFlagSet("rune "+kind, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tags := fs.String("tag", "", "comma-separated tags")
	body := fs.String("body", "", "body")
	fromStdin := fs.Bool("stdin", false, "stdin")
	pos, err := parseFlags(fs, args, map[string]bool{"tag": true, "body": true})
	if err != nil {
		return err
	}
	scope, store, err := scopedStore(cwd, false, "")
	if err != nil {
		return err
	}
	scope.Project = ""
	scope.Inbox = kind == "inbox"
	scope.Today = kind == "today"
	if len(pos) == 0 && !*fromStdin {
		items, _, err := store.Items(scope, core.ListOptions{})
		if err != nil {
			return err
		}
		printItems(stdout, scope.Home, items)
		return nil
	}
	if *fromStdin {
		text, err := readAll(stdin)
		if err != nil {
			return err
		}
		*body = text
	}
	item, err := store.Add(scope, core.AddOptions{
		Title:  core.DecodeEscapes(strings.Join(pos, " ")),
		Body:   core.DecodeEscapes(*body),
		Tags:   splitCSV(*tags),
		AsNote: true,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Added %s  %s\n", item.DisplayID, item.Title)
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
	fmt.Fprintf(stdout, "Restored %d item(s) into %s\n", count, core.ProjectPath(scope.Home, scope.Project))
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
	store := core.NewStore(scope.Home)
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

func readAll(r io.Reader) (string, error) {
	if r == nil {
		return "", nil
	}
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	return buf.String(), err
}

func printItems(w io.Writer, home string, items []*core.Item) {
	if len(items) == 0 {
		fmt.Fprintln(w, "No items.")
		return
	}
	for _, item := range items {
		box := "   "
		if item.Type == core.ItemTask {
			if item.Done {
				box = "[x]"
			} else {
				box = "[ ]"
			}
		}
		tags := ""
		if len(item.Tags) > 0 {
			tags = "  #" + strings.Join(item.Tags, " #")
		}
		source := item.Project
		if source == "" && item.Doc != nil {
			source = item.Doc.RelPath(home)
		}
		if source != "" {
			source = "  " + source
		}
		fmt.Fprintf(w, "%-4s %s %s%s%s\n", item.DisplayID, box, item.Title, tags, source)
	}
}

func printItemDetail(w io.Writer, home string, item *core.Item) {
	box := ""
	if item.Type == core.ItemTask {
		box = "[ ] "
		if item.Done {
			box = "[x] "
		}
	}
	fmt.Fprintf(w, "%s  %s%s\n", item.DisplayID, box, item.Title)
	if len(item.Tags) > 0 {
		fmt.Fprintf(w, "tags: %s\n", strings.Join(item.Tags, ","))
	}
	if item.Source != "" {
		source := item.Source
		if item.Doc != nil {
			source = item.Doc.RelPath(home)
		}
		fmt.Fprintf(w, "source: %s:%d\n", source, item.Line+1)
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
	fmt.Fprintf(w, "rune: %v\n", err)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Rune captures project memory from the terminal.

Usage:
  rune
  rune add "fix stuns" --tag combat,bug
  rune list [--global] [--all] [--done] [--tag tag]
  rune yank <id>
  rune edit <id> --end "details with \n newlines"
  rune done <id>
  rune find "query" --global

Commands:
  add, list, show, yank, edit, done, undone, toggle, tag, untag, find
  today, inbox, projects, tags, archive, restore, import, path, doctor`)
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
