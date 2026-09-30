package report

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Afrawles/devreport/internal/gitlog"
)

func commit(short, name, email, subject string, files ...string) gitlog.Commit {
	return gitlog.Commit{Hash: short + "000", Short: short, AuthorName: name, AuthorEmail: email,
		Subject: subject, Files: files, Date: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)}
}

func TestPathModule(t *testing.T) {
	cases := []struct {
		repo  string
		files []string
		want  string
	}{
		{"pidmis-backend", []string{"pidmis_gis/views_topology.py", "pidmis_gis/tests/test_x.py", "pidmis_assets/models.py"}, "GIS"},
		{"pidmis-frontend", []string{"src/features/water-network/editor/Editor.ts"}, "Water Network"},
		{"pidmis-frontend", []string{"src/components/pages/dashboard/Dash.tsx"}, "Dashboard"},
		{"pidmis-mobile", []string{"src/hooks/assets-and-inventory/forms/useX.tsx"}, "Assets and Inventory"},
		{"pidmis-backend", []string{".github/workflows/ci.yml", "Dockerfile"}, devopsModule},
		{"pidmis-backend", []string{"README.md"}, docsModule},
		{"pidmis-mobile", []string{"yarn.lock", "src/components/map/Picker.tsx"}, "Map"},
		{"pidmis-frontend", []string{"src/components/common/Table.tsx", "src/hooks/connections/useX.tsx"}, "Connections"},
		{"pidmis-frontend", []string{"src/components/common/Table.tsx", "src/utils/apis.ts"}, sharedModule},
		{"pidmis-backend", []string{"pidmis/settings.py"}, "Project Settings"},
	}
	for _, c := range cases {
		if got := PathModule(c.repo, c.files); got != c.want {
			t.Errorf("PathModule(%v) = %q, want %q", c.files, got, c.want)
		}
	}
}

func TestSubjectBullets(t *testing.T) {
	cs := []gitlog.Commit{
		commit("a", "x", "x", "feat(auth): require email+SMS OTP 2FA on login"),
		commit("b", "x", "x", "fix: backgroung otp"),
		commit("c", "x", "x", "Fix simulation report"),
		commit("d", "x", "x", "feat(auth): require email+SMS OTP 2FA on login"),
		commit("e", "x", "x", "wip"),
		commit("f", "x", "x", "remove unsed"),
	}
	got := SubjectBullets(cs, 6)
	want := []string{"Require email+SMS OTP 2FA on login", "Fixes: backgroung otp; simulation report"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestChallengeHints(t *testing.T) {
	cs := []gitlog.Commit{
		commit("a", "x", "x", "fix: prevent macOS objc fork crash in rq worker for simulation jobs"),
		commit("b", "x", "x", "Revert \"feat: pidmis swarm deploy\""),
		commit("c", "x", "x", "fix: spacing"),
		commit("d", "x", "x", "feat: add exports"),
	}
	got := ChallengeHints(cs)
	want := []string{"Prevent macOS objc fork crash in rq worker for simulation jobs – fixed", "Reverted: feat: pidmis swarm deploy"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", got)
	}
	if len(ChallengeHints(cs[2:])) != 0 {
		t.Fatal("ordinary commits should give no challenges")
	}
}

func TestIdentities(t *testing.T) {
	cs := []gitlog.Commit{
		commit("a", "izaiah-m", "imukisa@gmail.com", "x"),
		commit("b", "Izaiah Mukisa", "92892230+izaiah-m@users.noreply.github.com", "Merge pull request #1"),
		commit("c", "ian", "ian@x.com", "x"),
		commit("d", "Ian-Balijawa", "ian@x.com", "x"),
		commit("e", "Ian-Balijawa", "ian@x.com", "x"),
		commit("f", "JosephatJuma", "j@x.com", "x"),
		commit("g", "ConradKash", "c@x.com", "x"),
	}
	ids := NewIdentities(cs, map[string]string{"ConradKash": "Conrad Kakuru"})
	check := func(c gitlog.Commit, want string) {
		t.Helper()
		if got := ids.Name(c); got != want {
			t.Errorf("Name(%s) = %q, want %q", c.AuthorName, got, want)
		}
	}
	check(cs[0], "Izaiah Mukisa")
	check(cs[2], "Ian Balijawa")
	check(cs[5], "Josephat Juma")
	check(cs[6], "Conrad Kakuru")
}

type fakeLLM struct{ resp string }

func (f fakeLLM) Name() string                           { return "fake" }
func (f fakeLLM) Complete(prompt string) (string, error) { return f.resp, nil }

func TestGroupLLMWithFallback(t *testing.T) {
	cs := []gitlog.Commit{
		commit("a1", "Moses Odeke", "m@x.com", "feat(auth): 2FA", "pidmis_auth/otp.py"),
		commit("b2", "Moses Odeke", "m@x.com", "fix validation", "pidmis_auth/serializers.py"),
		commit("c3", "izaiah-m", "i@x.com", "feat: dashboards", "pidmis_reports/views.py"),
	}
	g := &ModuleGrouper{
		Mode:  GroupLLM,
		Names: NewIdentities(cs, nil),
		// c3 is left out on purpose and "zz" does not exist.
		LLM: fakeLLM{resp: "```json\n{\"modules\":[{\"name\":\"Authentication\",\"achievements\":[\"Two-factor login\",\"Fixes: login validation\"],\"challenges\":[\"OTP emails timed out – fixed\"],\"commit_ids\":[\"a1\",\"b2\",\"zz\"]}]}\n```"},
	}
	mods := g.Group("Backend", "pidmis-backend", cs)
	if len(mods) != 2 {
		t.Fatalf("want 2 modules, got %+v", mods)
	}
	if mods[0].Name != "Authentication" || len(mods[0].Commits) != 2 || mods[0].Developers[0] != "Moses Odeke" || len(mods[0].Challenges) != 1 {
		t.Fatalf("bad llm module: %+v", mods[0])
	}
	if mods[1].Name != "Reports" || mods[1].Developers[0] != "izaiah-m" {
		t.Fatalf("bad fallback module: %+v", mods[1])
	}
}

func TestReplaceFirstTable(t *testing.T) {
	doc := `<w:body><w:p/><w:tbl><w:tblPr/><w:tr><w:tc><w:tbl><w:tr/></w:tbl></w:tc></w:tr></w:tbl><w:p>after</w:p><w:tbl>second</w:tbl></w:body>`
	got, ok := replaceFirstTable(doc, "<NEW/>")
	want := `<w:body><w:p/><NEW/><w:p>after</w:p><w:tbl>second</w:tbl></w:body>`
	if !ok || got != want {
		t.Fatalf("got %q", got)
	}
}

func TestDocxExportIsWellFormed(t *testing.T) {
	r := ModuleReport{SubmittedBy: "Moses Odeke", Period: "July – September 2026", Year: 2026, Sections: []Section{{
		Title: "Backend (pidmis-backend)",
		Modules: []Module{{Name: "GIS <Import> & Export", Achievements: []string{"KML/KMZ import", "Fixes: ids"},
			Developers: []string{"Moses Odeke"}, CompletionDate: "17 Sep 2026"}},
	}}}
	path := filepath.Join(t.TempDir(), "r.docx")
	if err := (DocxExporter{}).Export(r, path); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		rc, _ := f.Open()
		dec := xml.NewDecoder(rc)
		for {
			if _, err := dec.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s is not well-formed XML: %v", f.Name, err)
			}
		}
		rc.Close()
	}

	// Template mode keeps the rest of the document.
	if tpl := os.Getenv("DOCX_TEMPLATE"); tpl != "" {
		out := filepath.Join(t.TempDir(), "t.docx")
		if err := (DocxExporter{TemplatePath: tpl}).Export(r, out); err != nil {
			t.Fatal(err)
		}
	}
}
