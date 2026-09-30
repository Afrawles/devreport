package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/gitlog"
	"github.com/Afrawles/devreport/internal/llm"
)

// ModuleReport is the data behind the module-grouped (docx) report. It is also
// exported as JSON so it can be edited by hand and re-rendered with --input.
type ModuleReport struct {
	Dept        string    `json:"dept"`
	SubmittedBy string    `json:"submitted_by"`
	Period      string    `json:"period"`
	Year        int       `json:"year"`
	Sections    []Section `json:"sections"`
}

// Section is one codebase/layer, e.g. Backend, Web, Mobile.
type Section struct {
	Title   string   `json:"title"`
	Modules []Module `json:"modules"`
}

type Module struct {
	Name           string   `json:"name"`
	Achievements   []string `json:"achievements"`
	Developers     []string `json:"developers"`
	Challenges     []string `json:"challenges,omitempty"`
	SupportFrom    string   `json:"support_from,omitempty"`
	FollowUp       []string `json:"follow_up,omitempty"`
	CompletionDate string   `json:"completion_date"`
	Commits        []string `json:"commits"`
}

func (r ModuleReport) SaveJSON(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadModuleReport(path string) (ModuleReport, error) {
	var r ModuleReport
	data, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(data, &r)
	return r, err
}

// ---------------------------------------------------------------------------
// Developer identities
// ---------------------------------------------------------------------------

// Identities maps commit authors to one display name per person: commits with
// the same email are merged, full names ("Izaiah Mukisa") win over handles
// ("izaiah-m"), GitHub noreply emails on merge commits link a handle to the
// full name, and explicit aliases override everything.
type Identities struct {
	byEmail map[string]string
	byName  map[string]string
	byLogin map[string]string
	aliases map[string]string
}

var noreplyEmail = regexp.MustCompile(`^(?:\d+\+)?([^@]+)@users\.noreply\.github\.com$`)

// NewIdentities builds the mapping. Pass every commit you have, merges
// included: merges made on GitHub carry the full name next to the login.
// aliases keys may be an author name, a handle or an email
// (case-insensitive); values are the display name.
func NewIdentities(commits []gitlog.Commit, aliases map[string]string) *Identities {
	id := &Identities{
		byEmail: map[string]string{},
		byName:  map[string]string{},
		byLogin: map[string]string{},
		aliases: map[string]string{},
	}
	for k, v := range aliases {
		id.aliases[norm(k)] = strings.TrimSpace(v)
	}

	counts := map[string]map[string]int{}
	for _, c := range commits {
		if counts[c.AuthorEmail] == nil {
			counts[c.AuthorEmail] = map[string]int{}
		}
		counts[c.AuthorEmail][c.AuthorName]++
	}

	for email, names := range counts {
		if a, ok := id.aliases[norm(email)]; ok {
			id.byEmail[email] = a
			continue
		}
		best, bestScore := "", -1
		for name, n := range names {
			if a, ok := id.aliases[norm(name)]; ok {
				best, bestScore = a, 1<<30
				break
			}
			score := n
			if isFullName(name) {
				score += 1 << 20 // prefer real names over handles
			}
			if score > bestScore || (score == bestScore && name < best) {
				best, bestScore = name, score
			}
		}
		id.byEmail[email] = best

		if m := noreplyEmail.FindStringSubmatch(email); m != nil && isFullName(best) {
			id.byLogin[norm(m[1])] = best
		}
	}

	// Same name under different emails is the same person.
	for _, name := range id.byEmail {
		if _, ok := id.byName[norm(name)]; !ok {
			id.byName[norm(name)] = name
		}
	}
	return id
}

func (id *Identities) Name(c gitlog.Commit) string {
	if a, ok := id.aliases[norm(c.AuthorName)]; ok {
		return a
	}
	if a, ok := id.aliases[norm(c.AuthorEmail)]; ok {
		return a
	}
	name := id.byEmail[c.AuthorEmail]
	if name == "" {
		name = c.AuthorName
	}
	if a, ok := id.aliases[norm(name)]; ok {
		return a
	}
	if !isFullName(name) {
		for _, handle := range []string{name, c.AuthorName} {
			if full, ok := id.byLogin[norm(handle)]; ok {
				return full
			}
		}
	}
	if canonical, ok := id.byName[norm(name)]; ok {
		name = canonical
	}
	return prettifyHandle(name)
}

func isFullName(s string) bool {
	return strings.Contains(strings.TrimSpace(s), " ")
}

var (
	camelHandle = regexp.MustCompile(`^([A-Z][a-z]+)([A-Z][a-z]+)$`)
	sepHandle   = regexp.MustCompile(`^([A-Z][a-z]+)[-_.]([A-Z][a-z]+)$`)
)

// prettifyHandle turns "JosephatJuma" or "Ian-Balijawa" into "Josephat Juma"
// / "Ian Balijawa"; anything else is returned unchanged.
func prettifyHandle(s string) string {
	if m := camelHandle.FindStringSubmatch(s); m != nil {
		return m[1] + " " + m[2]
	}
	if m := sepHandle.FindStringSubmatch(s); m != nil {
		return m[1] + " " + m[2]
	}
	return s
}

func norm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// ---------------------------------------------------------------------------
// Grouping
// ---------------------------------------------------------------------------

const (
	GroupLLM  = "llm"
	GroupPath = "path"
)

type ModuleGrouper struct {
	LLM        llm.Provider
	Mode       string // GroupLLM or GroupPath
	BatchSize  int
	MaxBullets int
	Names      *Identities
}

// Group turns a section's commits into modules. In llm mode the model decides
// the modules and writes the bullets; any batch it fails on, and any commit it
// leaves out, falls back to path-based grouping so nothing is dropped.
func (g *ModuleGrouper) Group(sectionName, repoName string, commits []gitlog.Commit) []Module {
	if g.BatchSize <= 0 {
		g.BatchSize = 60
	}
	if g.MaxBullets < 2 {
		g.MaxBullets = 8
	}

	byShort := map[string]gitlog.Commit{}
	for _, c := range commits {
		byShort[c.Short] = c
	}

	type draft struct {
		name         string
		achievements []string
		challenges   []string
		commits      []string
	}
	var drafts []*draft
	index := map[string]*draft{}
	add := func(name string, achievements, challenges []string, ids []string) {
		key := norm(name)
		d, ok := index[key]
		if !ok {
			d = &draft{name: name}
			index[key] = d
			drafts = append(drafts, d)
		}
		d.achievements = append(d.achievements, achievements...)
		d.challenges = append(d.challenges, challenges...)
		d.commits = append(d.commits, ids...)
	}

	placed := map[string]bool{}
	if g.Mode == GroupLLM && g.LLM != nil {
		for start := 0; start < len(commits); start += g.BatchSize {
			end := min(start+g.BatchSize, len(commits))
			var known []string
			for _, d := range drafts {
				known = append(known, d.name)
			}
			mods, err := g.llmBatch(sectionName, commits[start:end], known)
			if err != nil {
				fmt.Printf("  LLM grouping failed for %s (commits %d-%d), using path grouping: %v\n", sectionName, start, end, err)
				continue
			}
			for _, m := range mods {
				var ids []string
				for _, id := range m.CommitIDs {
					id = strings.TrimSpace(id)
					if c, ok := byShort[id]; ok && !placed[c.Short] {
						placed[c.Short] = true
						ids = append(ids, c.Short)
					}
				}
				if len(ids) == 0 {
					continue
				}
				add(m.Name, m.Achievements, m.Challenges, ids)
			}
		}
	}

	// Path grouping for everything not placed by the LLM.
	pathGroups := map[string][]gitlog.Commit{}
	var pathOrder []string
	for _, c := range commits {
		if placed[c.Short] {
			continue
		}
		name := PathModule(repoName, c.Files)
		if _, ok := pathGroups[name]; !ok {
			pathOrder = append(pathOrder, name)
		}
		pathGroups[name] = append(pathGroups[name], c)
	}
	for _, name := range pathOrder {
		group := pathGroups[name]
		var ids []string
		for _, c := range group {
			ids = append(ids, c.Short)
		}
		add(name, SubjectBullets(group, g.MaxBullets), ChallengeHints(group), ids)
	}

	var modules []Module
	for _, d := range drafts {
		var cs []gitlog.Commit
		for _, id := range d.commits {
			cs = append(cs, byShort[id])
		}
		modules = append(modules, Module{
			Name:           d.name,
			Achievements:   dedupe(d.achievements),
			Challenges:     dedupe(d.challenges),
			Developers:     g.developers(cs),
			CompletionDate: latest(cs).Format("02 Jan 2006"),
			Commits:        d.commits,
		})
	}

	sort.SliceStable(modules, func(i, j int) bool {
		return len(modules[i].Commits) > len(modules[j].Commits)
	})
	return modules
}

func (g *ModuleGrouper) developers(cs []gitlog.Commit) []string {
	counts := map[string]int{}
	for _, c := range cs {
		counts[g.Names.Name(c)]++
	}
	var names []string
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	return names
}

type llmModule struct {
	Name         string   `json:"name"`
	Achievements []string `json:"achievements"`
	Challenges   []string `json:"challenges"`
	CommitIDs    []string `json:"commit_ids"`
}

func (g *ModuleGrouper) llmBatch(section string, batch []gitlog.Commit, known []string) ([]llmModule, error) {
	var sb strings.Builder
	sb.WriteString("You are writing a software progress report. Group the git commits below from the ")
	sb.WriteString(section)
	sb.WriteString(" codebase into feature modules.\n\nRules:\n")
	sb.WriteString("- Group by functional module / feature area (e.g. Authentication, Assets Management, GIS Hydraulic Simulation, Reports & Dashboards, DevOps & Deployment), NOT by commit type.\n")
	sb.WriteString("- Put bug fixes, refactors and small chores in the same module as the feature they touch.\n")
	sb.WriteString("- Use the file paths to decide the module when the commit message is vague.\n")
	if len(known) > 0 {
		sb.WriteString("- Reuse these existing module names when a commit belongs to one of them: " + strings.Join(known, "; ") + "\n")
	}
	sb.WriteString(fmt.Sprintf("- For each module write 1 to %d short achievement bullets describing what was built or fixed. Combine related commits into one bullet. Put fixes into a single bullet starting with \"Fixes: \".\n", g.MaxBullets))
	sb.WriteString("- Bullets are short noun phrases a manager can read (\"Two-factor login with email + SMS OTP\"), keep technical names that matter, no trailing full stop, no developer names.\n")
	sb.WriteString("- Only describe work that is in the commits. Every commit id must appear in exactly one module.\n")
	sb.WriteString("- \"challenges\": list a challenge ONLY when the commits show a real problem the team hit: a crash, a revert or rollback, a workaround, a data or schema mismatch, a performance problem, or the same thing being fixed again and again. " +
		"Write the problem and whether it was resolved, e.g. \"Background worker crashed on macOS when running simulation jobs – fixed\". " +
		"Ordinary bug fixes are not challenges. When there is no such evidence use an empty list; never guess.\n\n")
	sb.WriteString("Respond with ONLY valid JSON, no markdown fences, in this exact shape:\n")
	sb.WriteString(`{"modules":[{"name":"Module name","achievements":["bullet 1","bullet 2"],"challenges":[],"commit_ids":["abc1234"]}]}`)
	sb.WriteString("\n\nCommits:\n")
	for _, c := range batch {
		sb.WriteString(fmt.Sprintf("- id: %s | subject: %s | paths: %s\n", c.Short, c.Subject, strings.Join(topPaths(c.Files, 6), ", ")))
	}

	resp, err := g.LLM.Complete(sb.String())
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Modules []llmModule `json:"modules"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(resp)), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse module response: %w", err)
	}
	if len(parsed.Modules) == 0 {
		return nil, fmt.Errorf("model returned no modules")
	}
	return parsed.Modules, nil
}

// ---------------------------------------------------------------------------
// Path heuristics
// ---------------------------------------------------------------------------

var containerDirs = map[string]bool{
	"src": true, "lib": true, "app": true, "apps": true, "internal": true, "pkg": true, "cmd": true,
	"features": true, "feature": true, "modules": true, "module": true, "components": true,
	"component": true, "hooks": true, "pages": true, "page": true, "screens": true, "screen": true,
	"views": true, "packages": true, "main": true, "java": true, "kotlin": true,
}

const (
	devopsModule = "Build, CI & Deployment"
	docsModule   = "Documentation"
	generalMod   = "General"
)

// Commit messages too vague to be worth a bullet on their own.
var vagueSubjects = map[string]bool{
	"added": true, "add": true, "update": true, "updates": true, "updated": true, "cleanup": true,
	"clean up": true, "cleaned code": true, "code cleanup": true, "remove unused": true,
	"remove unsed": true, "unsed": true, "unused": true, "wip": true, "minor": true, "minor fixes": true,
	"refactor": true, "refactoring": true, "changes": true, "misc": true, "format": true, "lint": true,
	"remove unused comments": true, "remove unsed comments": true,
}

var lockFiles = map[string]bool{
	"yarn.lock": true, "package-lock.json": true, "pnpm-lock.yaml": true, "go.sum": true,
	"poetry.lock": true, "Podfile.lock": true, "Gemfile.lock": true,
}

// Generic folders that say little about the feature; a commit is only filed
// under "Shared Components & Utilities" when it touches nothing more specific.
var weakDirs = map[string]bool{
	"common": true, "shared": true, "utils": true, "util": true, "helpers": true, "helper": true,
	"handlers": true, "constants": true, "config": true, "types": true, "@types": true, "context": true,
	"stepper": true, "layouts": true, "layout": true, "charts": true, "assets": true, "styles": true,
	"css": true, "img": true, "images": true, "icons": true, "svg": true, "menu": true, "inputs": true,
	"core": true, "store": true, "theme": true,
}

const sharedModule = "Shared Components & Utilities"

// PathModule picks a module name for a commit from the files it touched: the
// most common meaningful directory, ignoring generic containers like src/ or
// components/ and the repo's own prefix (pidmis_gis -> GIS).
func PathModule(repoName string, files []string) string {
	strong := map[string]int{}
	var order []string
	infra, docs, weak := 0, 0, 0
	for _, f := range files {
		base := filepath.Base(f)
		switch {
		case lockFiles[base]:
			continue
		case isInfra(f):
			infra++
			continue
		case strings.HasSuffix(strings.ToLower(base), ".md"):
			docs++
			continue
		}
		key, isWeak := moduleKey(repoName, f)
		if isWeak {
			weak++
			continue
		}
		if _, ok := strong[key]; !ok {
			order = append(order, key)
		}
		strong[key]++
	}

	switch {
	case len(strong) > 0:
		best := order[0]
		for _, k := range order {
			if strong[k] > strong[best] {
				best = k
			}
		}
		return best
	case weak > 0:
		return sharedModule
	case infra > 0:
		return devopsModule
	case docs > 0:
		return docsModule
	default:
		return devopsModule // only lock files
	}
}

func isInfra(f string) bool {
	lower := strings.ToLower(f)
	base := filepath.Base(lower)
	prefixes := []string{".github/", ".gitlab", ".circleci/", "android/", "ios/", "scripts/deploy", "deploy/", "k8s/", "helm/", ".zed/", ".vscode/", ".idea/"}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	names := []string{"dockerfile", "makefile", "entrypoint.sh", "package.json", "requirements.txt", "go.mod",
		"tsconfig.json", ".gitignore", "procfile", "app.json", "babel.config.js", "metro.config.js"}
	for _, n := range names {
		if base == n {
			return true
		}
	}
	return strings.HasPrefix(base, "docker-compose") || strings.HasPrefix(base, "docker-stack")
}

// moduleKey returns the module name for one file and whether it came from a
// generic (weak) folder.
func moduleKey(repoName string, f string) (string, bool) {
	dir := filepath.ToSlash(filepath.Dir(f))
	if dir == "." {
		return sharedModule, true
	}
	for _, seg := range strings.Split(dir, "/") {
		lower := strings.ToLower(seg)
		if containerDirs[lower] || strings.HasPrefix(seg, ".") {
			continue
		}
		if weakDirs[lower] {
			return sharedModule, true
		}
		if lower == repoToken(repoName) {
			return "Project Settings", false // e.g. pidmis/settings.py
		}
		return humanize(stripRepoPrefix(repoName, seg)), false
	}
	return sharedModule, true
}

func repoToken(repoName string) string {
	parts := strings.FieldsFunc(repoName, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(parts[0])
}

func stripRepoPrefix(repoName, seg string) string {
	token := repoToken(repoName)
	lower := strings.ToLower(seg)
	for _, sep := range []string{"_", "-"} {
		if strings.HasPrefix(lower, token+sep) && len(seg) > len(token)+1 {
			return seg[len(token)+1:]
		}
	}
	return seg
}

var smallWords = map[string]bool{"and": true, "of": true, "the": true, "for": true, "to": true, "in": true}

func humanize(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, w := range words {
		lw := strings.ToLower(w)
		switch {
		case i > 0 && smallWords[lw]:
			words[i] = lw
		case len(w) <= 3 && lw != "map" && lw != "app":
			words[i] = strings.ToUpper(w) // gis -> GIS, api -> API
		default:
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	if len(words) == 0 {
		return generalMod
	}
	return strings.Join(words, " ")
}

func topPaths(files []string, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		dir := filepath.ToSlash(filepath.Dir(f))
		parts := strings.Split(dir, "/")
		if len(parts) > 3 {
			parts = parts[:3]
		}
		p := strings.Join(parts, "/")
		if p == "." {
			p = filepath.Base(f)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Commit subjects -> bullets (path mode, and LLM fallback)
// ---------------------------------------------------------------------------

var conventional = regexp.MustCompile(`^\s*([A-Za-z]+)(\([^)]*\))?!?:\s*`)

// SubjectBullets turns commit subjects into report bullets: conventional
// prefixes are stripped, duplicates removed, and all fixes collapsed into one
// "Fixes: ..." bullet. At most max bullets are returned.
func SubjectBullets(commits []gitlog.Commit, max int) []string {
	var features, fixes []string
	seen := map[string]bool{}

	// oldest first reads more naturally
	sorted := append([]gitlog.Commit(nil), commits...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })

	for _, c := range sorted {
		subject := strings.TrimSpace(strings.SplitN(c.Subject, "  ", 2)[0])
		kind := ""
		if m := conventional.FindStringSubmatch(subject); m != nil {
			kind = strings.ToLower(m[1])
			subject = subject[len(m[0]):]
		}
		subject = strings.TrimRight(strings.TrimSpace(subject), ".")
		if len(subject) < 4 || vagueSubjects[strings.ToLower(subject)] || strings.HasPrefix(strings.ToLower(subject), "index on ") {
			continue
		}
		if kind == "" && strings.HasPrefix(strings.ToLower(subject), "fix") {
			kind = "fix"
		}
		key := strings.ToLower(subject)
		if seen[key] {
			continue
		}
		seen[key] = true
		subject = strings.ToUpper(subject[:1]) + subject[1:]
		if kind == "fix" {
			fixes = append(fixes, lowerFirst(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(subject, "Fix:"), "Fix "))))
		} else {
			features = append(features, subject)
		}
	}

	limit := max
	if len(fixes) > 0 {
		limit = max - 1
	}
	var out []string
	if len(features) > limit {
		extra := len(features) - (limit - 1)
		out = append(out, features[:limit-1]...)
		out = append(out, fmt.Sprintf("…and %d other changes", extra))
	} else {
		out = append(out, features...)
	}
	if len(fixes) > 0 {
		if len(fixes) > 6 {
			fixes = append(fixes[:6], fmt.Sprintf("%d more", len(fixes)-6))
		}
		out = append(out, "Fixes: "+strings.Join(fixes, "; "))
	}
	return out
}

// lowerFirst lower-cases the first letter unless the word looks like an
// acronym (KML, OTP).
func lowerFirst(s string) string {
	if len(s) < 2 || strings.ToUpper(s[:2]) == s[:2] {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

var challengeWords = regexp.MustCompile(`(?i)\b(revert(ed|s)?|roll ?back|crash(es|ed|ing)?|hot ?fix|workaround|deadlock|race condition|memory leak|time ?outs?|timed out|drift|outage|downtime|broke(n)?)\b`)

// ChallengeHints picks out commits that point at a real problem (crashes,
// reverts, rollbacks, workarounds...) for the Challenges column when no LLM
// is used. It returns at most three, and nothing when there is no evidence.
func ChallengeHints(commits []gitlog.Commit) []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range commits {
		subject := strings.TrimSpace(strings.SplitN(c.Subject, "  ", 2)[0])
		if !challengeWords.MatchString(subject) {
			continue
		}
		fixed := false
		if m := conventional.FindStringSubmatch(subject); m != nil {
			fixed = strings.EqualFold(m[1], "fix")
			subject = subject[len(m[0]):]
		}
		if strings.HasPrefix(strings.ToLower(subject), "revert ") {
			subject = "Reverted: " + strings.Trim(subject[len("revert "):], `" `)
		} else if strings.HasPrefix(strings.ToLower(subject), "fix") {
			fixed = true
			subject = strings.TrimSpace(strings.TrimLeft(subject[3:], ":ed "))
		}
		subject = strings.TrimRight(strings.TrimSpace(subject), ".")
		if subject == "" || seen[strings.ToLower(subject)] {
			continue
		}
		seen[strings.ToLower(subject)] = true
		subject = strings.ToUpper(subject[:1]) + subject[1:]
		if fixed {
			subject += " – fixed"
		}
		out = append(out, subject)
		if len(out) == 3 {
			break
		}
	}
	return out
}

func dedupe(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range items {
		s = strings.TrimSpace(s)
		if s == "" || seen[strings.ToLower(s)] {
			continue
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	return out
}

func latest(cs []gitlog.Commit) time.Time {
	var t time.Time
	for _, c := range cs {
		if c.Date.After(t) {
			t = c.Date
		}
	}
	return t
}

// PeriodLabel renders a date range the way the report template expects:
// "September 2026", "July – September 2026" or "Nov 2025 – Feb 2026".
func PeriodLabel(start, end time.Time) string {
	switch {
	case start.Year() == end.Year() && start.Month() == end.Month():
		return end.Format("January 2006")
	case start.Year() == end.Year():
		return start.Format("January") + " – " + end.Format("January 2006")
	default:
		return start.Format("Jan 2006") + " – " + end.Format("Jan 2006")
	}
}
