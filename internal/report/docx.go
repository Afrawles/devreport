package report

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

// Column widths in twentieths of a point (dxa). They add up to the width of
// the "Individual Report" template table so a generated table drops straight
// into that template.
var docxColumns = []int{2000, 5100, 1700, 1780, 1100, 1400, 1300}

const (
	docxTableWidth = 14380
	docxHeaderFill = "FFC000"
	docxBandFill   = "D9E2F3"
	docxFont       = `<w:rFonts w:ascii="Trebuchet MS" w:eastAsia="Times New Roman" w:hAnsi="Trebuchet MS" w:cs="Times New Roman"/>`
	docxBorders    = `<w:top w:val="single" w:sz="8" w:space="0" w:color="auto"/><w:left w:val="single" w:sz="8" w:space="0" w:color="auto"/><w:bottom w:val="single" w:sz="8" w:space="0" w:color="auto"/><w:right w:val="single" w:sz="8" w:space="0" w:color="auto"/>`
)

type DocxExporter struct {
	// TemplatePath, when set, is a .docx whose first table is replaced by the
	// generated report table; everything else (header, notes, page setup)
	// is kept as is.
	TemplatePath string
}

func (e DocxExporter) Export(r ModuleReport, path string) error {
	table := buildReportTable(r)

	var data []byte
	var err error
	if e.TemplatePath != "" {
		data, err = fillTemplate(e.TemplatePath, table)
	} else {
		data, err = standaloneDocx(r, table)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ---------------------------------------------------------------------------
// Table XML
// ---------------------------------------------------------------------------

func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			// drop characters that are illegal in XML 1.0
			if r == 0x9 || r == 0xA || r == 0xD || r >= 0x20 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

type runOpts struct {
	bold      bool
	size      int // half-points
	highlight string
}

func docxRun(text string, o runOpts) string {
	if o.size == 0 {
		o.size = 18
	}
	rpr := docxFont
	if o.bold {
		rpr += "<w:b/><w:bCs/>"
	}
	rpr += `<w:color w:val="000000"/>`
	if o.highlight != "" {
		rpr += fmt.Sprintf(`<w:highlight w:val="%s"/>`, o.highlight)
	}
	rpr += fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, o.size, o.size)
	return fmt.Sprintf(`<w:r><w:rPr>%s</w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, rpr, xmlEscape(text))
}

func docxPara(run string, center, bullet bool) string {
	ppr := `<w:spacing w:before="20" w:after="40" w:line="240" w:lineRule="auto"/>`
	if bullet {
		ppr += `<w:ind w:left="227" w:hanging="170"/>`
	}
	if center {
		ppr += `<w:jc w:val="center"/>`
	}
	return fmt.Sprintf(`<w:p><w:pPr>%s</w:pPr>%s</w:p>`, ppr, run)
}

func textParas(lines []string, o runOpts) []string {
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, docxPara(docxRun(l, o), false, false))
		}
	}
	return out
}

func bulletParas(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, docxPara(docxRun("• "+l, runOpts{}), false, true))
		}
	}
	return out
}

func docxCell(width int, paras []string, fill string, span int, vAlign string) string {
	tcpr := fmt.Sprintf(`<w:tcW w:w="%d" w:type="dxa"/>`, width)
	if span > 1 {
		tcpr += fmt.Sprintf(`<w:gridSpan w:val="%d"/>`, span)
	}
	tcpr += "<w:tcBorders>" + docxBorders + "</w:tcBorders>"
	if fill != "" {
		tcpr += fmt.Sprintf(`<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, fill)
	}
	if vAlign == "" {
		vAlign = "top"
	}
	tcpr += fmt.Sprintf(`<w:tcMar><w:left w:w="80" w:type="dxa"/><w:right w:w="80" w:type="dxa"/></w:tcMar><w:vAlign w:val="%s"/>`, vAlign)
	if len(paras) == 0 {
		paras = []string{docxPara("", false, false)}
	}
	return fmt.Sprintf(`<w:tc><w:tcPr>%s</w:tcPr>%s</w:tc>`, tcpr, strings.Join(paras, ""))
}

func docxRow(cells []string, header, cantSplit bool) string {
	trpr := ""
	if header {
		trpr += "<w:tblHeader/>"
	}
	if cantSplit {
		trpr += "<w:cantSplit/>"
	}
	return fmt.Sprintf(`<w:tr><w:trPr>%s</w:trPr>%s</w:tr>`, trpr, strings.Join(cells, ""))
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func buildReportTable(r ModuleReport) string {
	W := docxColumns
	bold := runOpts{bold: true}
	var rows []string

	dept := r.Dept
	deptOpts := bold
	if strings.TrimSpace(dept) == "" {
		dept, deptOpts.highlight = "XXXXXXX", "red"
	}
	submitted := r.SubmittedBy
	submittedOpts := bold
	if strings.TrimSpace(submitted) == "" {
		submitted, submittedOpts.highlight = "XXXXXXXX", "red"
	}

	rows = append(rows, docxRow([]string{
		docxCell(W[0], []string{docxPara(docxRun("Dept:", bold), false, false)}, docxHeaderFill, 1, "center"),
		docxCell(W[1], []string{docxPara(docxRun(dept, deptOpts), false, false)}, docxHeaderFill, 1, "center"),
		docxCell(W[2], []string{docxPara(docxRun("Submitted by:", bold), false, false)}, docxHeaderFill, 1, "center"),
		docxCell(sum(W[3:]), []string{docxPara(docxRun(submitted, submittedOpts), false, false)}, docxHeaderFill, 4, "center"),
	}, false, false))

	rows = append(rows, docxRow([]string{
		docxCell(W[0], []string{docxPara(docxRun("PERIOD", bold), false, false)}, docxHeaderFill, 1, "center"),
		docxCell(W[1], []string{docxPara(docxRun(r.Period, bold), false, false)}, docxHeaderFill, 1, "center"),
		docxCell(sum(W[2:]), nil, docxHeaderFill, 5, ""),
	}, false, false))

	heads := []string{"KEY ACTIVITIES / TASKS (MODULE)", "ACHIEVEMENTS", "DEVELOPER(S)", "CHALLENGES ENCOUNTERED",
		"SUPPORT FROM (WHOM/ WHICH DEPT)", "FOLLOW UP ACTIVITIES", "COMPLETION DATE"}
	var headCells []string
	for i, h := range heads {
		headCells = append(headCells, docxCell(W[i], []string{docxPara(docxRun(h, runOpts{bold: true, size: 16}), true, false)}, docxHeaderFill, 1, "center"))
	}
	rows = append(rows, docxRow(headCells, true, false))

	for _, sec := range r.Sections {
		if len(r.Sections) > 1 || sec.Title != "" {
			rows = append(rows, docxRow([]string{
				docxCell(docxTableWidth, []string{docxPara(docxRun(sec.Title, runOpts{bold: true, size: 20}), false, false)}, docxBandFill, 7, ""),
			}, false, true))
		}
		for _, m := range sec.Modules {
			rows = append(rows, docxRow([]string{
				docxCell(W[0], []string{docxPara(docxRun(m.Name, bold), false, false)}, "", 1, ""),
				docxCell(W[1], bulletParas(m.Achievements), "", 1, ""),
				docxCell(W[2], textParas(m.Developers, runOpts{}), "", 1, ""),
				docxCell(W[3], textParas(m.Challenges, runOpts{}), "", 1, ""),
				docxCell(W[4], textParas([]string{m.SupportFrom}, runOpts{}), "", 1, ""),
				docxCell(W[5], textParas(m.FollowUp, runOpts{}), "", 1, ""),
				docxCell(W[6], []string{docxPara(docxRun(m.CompletionDate, runOpts{}), true, false)}, "", 1, ""),
			}, false, true))
		}
	}

	var grid strings.Builder
	for _, w := range W {
		grid.WriteString(fmt.Sprintf(`<w:gridCol w:w="%d"/>`, w))
	}

	return fmt.Sprintf(`<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblInd w:w="-1090" w:type="dxa"/>`+
		`<w:tblLayout w:type="fixed"/><w:tblLook w:val="04A0" w:firstRow="1" w:lastRow="0" w:firstColumn="1" w:lastColumn="0" w:noHBand="0" w:noVBand="1"/></w:tblPr>`+
		`<w:tblGrid>%s</w:tblGrid>%s</w:tbl>`, docxTableWidth, grid.String(), strings.Join(rows, ""))
}

// ---------------------------------------------------------------------------
// Template mode
// ---------------------------------------------------------------------------

func fillTemplate(templatePath, table string) ([]byte, error) {
	zr, err := zip.OpenReader(templatePath)
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer zr.Close()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	replaced := false

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}

		if f.Name == "word/document.xml" {
			doc, ok := replaceFirstTable(string(data), table)
			if !ok {
				return nil, fmt.Errorf("template has no table to replace")
			}
			data = []byte(doc)
			replaced = true
		}

		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: f.Modified})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if !replaced {
		return nil, fmt.Errorf("template has no word/document.xml")
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// replaceFirstTable swaps the first top-level <w:tbl> (including any nested
// tables inside it) for replacement.
func replaceFirstTable(doc, replacement string) (string, bool) {
	start := indexTableOpen(doc, 0)
	if start < 0 {
		return doc, false
	}
	depth, i := 0, start
	for i < len(doc) {
		open := indexTableOpen(doc, i)
		closeIdx := strings.Index(doc[i:], "</w:tbl>")
		if closeIdx < 0 {
			return doc, false
		}
		closeIdx += i
		if open >= 0 && open < closeIdx {
			depth++
			i = open + len("<w:tbl")
			continue
		}
		depth--
		i = closeIdx + len("</w:tbl>")
		if depth == 0 {
			return doc[:start] + replacement + doc[i:], true
		}
	}
	return doc, false
}

// indexTableOpen finds "<w:tbl>" or "<w:tbl " (not <w:tblPr>, <w:tblGrid>...).
func indexTableOpen(doc string, from int) int {
	for i := from; i < len(doc); {
		j := strings.Index(doc[i:], "<w:tbl")
		if j < 0 {
			return -1
		}
		j += i
		k := j + len("<w:tbl")
		if k < len(doc) && (doc[k] == '>' || doc[k] == ' ') {
			return j
		}
		i = k
	}
	return -1
}

// ---------------------------------------------------------------------------
// Standalone document (no template)
// ---------------------------------------------------------------------------

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`

const docxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`

const docxDocRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/></Relationships>`

const docxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Trebuchet MS" w:hAnsi="Trebuchet MS" w:eastAsia="Times New Roman" w:cs="Times New Roman"/><w:sz w:val="20"/><w:szCs w:val="20"/><w:lang w:val="en-GB"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style><w:style w:type="table" w:default="1" w:styleId="TableNormal"><w:name w:val="Normal Table"/><w:tblPr><w:tblInd w:w="0" w:type="dxa"/><w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="108" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style></w:styles>`

var docxHeaderTmpl = template.Must(template.New("h").Parse(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:shd w:val="clear" w:color="auto" w:fill="4472C4"/><w:spacing w:before="60" w:after="60"/><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/><w:bCs/><w:color w:val="FFFFFF"/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr><w:t>INDIVIDUAL REPORT {{.}}</w:t></w:r></w:p></w:hdr>`))

var docxDocumentTmpl = template.Must(template.New("d").Parse(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>{{.Table}}<w:p><w:pPr><w:spacing w:before="240"/></w:pPr><w:r><w:rPr><w:b/><w:bCs/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t>Note:</w:t></w:r><w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:br/><w:t xml:space="preserve">If applicable, please </w:t></w:r><w:r><w:rPr><w:b/><w:bCs/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t>attach or include links to any supporting documents, reports, photos, or records</w:t></w:r><w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve"> that provide additional evidence of the activities and achievements listed above.</w:t></w:r></w:p><w:sectPr><w:headerReference w:type="default" r:id="rId2"/><w:pgSz w:w="15840" w:h="12240" w:orient="landscape"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/><w:cols w:space="720"/></w:sectPr></w:body></w:document>`))

func standaloneDocx(r ModuleReport, table string) ([]byte, error) {
	var doc, header bytes.Buffer
	if err := docxDocumentTmpl.Execute(&doc, map[string]any{"Table": table}); err != nil {
		return nil, err
	}
	// html/template would escape the table; text/template does not, and the
	// year is an int so it needs no escaping.
	if err := docxHeaderTmpl.Execute(&header, r.Year); err != nil {
		return nil, err
	}

	parts := []struct{ name, body string }{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRootRels},
		{"word/_rels/document.xml.rels", docxDocRels},
		{"word/styles.xml", docxStyles},
		{"word/header1.xml", header.String()},
		{"word/document.xml", doc.String()},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range parts {
		w, err := zw.Create(p.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(p.body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
