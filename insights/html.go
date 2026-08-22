package insights

import (
	"html/template"
	"io"
	"net/url"
)

type htmlAction struct {
	Method      string
	Target      string
	Directories []string
	Pairs       []htmlPair
	Command     string
	Steps       []string
	Notes       string
}

type htmlPair struct {
	Kept         string
	KeptURL      template.URL
	Redundant    string
	RedundantURL template.URL
	Notes        string
}

type htmlOpportunity struct {
	Rank              int
	Title             string
	Category          string
	Directories       []string
	EstimatedHuman    string
	Confidence        string
	Description       string
	Evidence          []string
	VerificationSteps []string
	Actions           []htmlAction
	FileURLs          []template.URL
}

type htmlDoc struct {
	Root          string
	GeneratedAt   string
	UsageLine     string
	Summary       string
	Opportunities []htmlOpportunity
}

// WriteHTML renders r as a single self-contained HTML file — an
// overview table plus one detail card per opportunity, each
// directory linked as file:// — with no external script or
// stylesheet dependency. Like WriteMarkdown, this is a pure function
// of r: the write-up can be regenerated any time without redoing the
// analysis.
func WriteHTML(w io.Writer, r Report) error {
	doc := htmlDoc{
		Root:        r.Root,
		GeneratedAt: r.GeneratedAt.Format("2006-01-02 15:04:05 MST"),
		Summary:     r.Summary,
	}
	if r.UsedBytes > 0 && r.CapacityBytes > 0 {
		doc.UsageLine = humanBytes(r.UsedBytes) + " used of " + humanBytes(r.CapacityBytes)
	}

	for _, o := range r.Opportunities {
		ho := htmlOpportunity{
			Rank:              o.Rank,
			Title:             o.Title,
			Category:          string(o.Category),
			Directories:       o.Directories,
			EstimatedHuman:    humanBytes(o.EstimatedBytes),
			Confidence:        string(o.Confidence),
			Description:       o.Description,
			Evidence:          o.Evidence,
			VerificationSteps: o.VerificationSteps,
		}
		for _, d := range o.Directories {
			ho.FileURLs = append(ho.FileURLs, fileURL(d))
		}
		for _, a := range o.Actions {
			ha := htmlAction{
				Method:      string(a.Method),
				Target:      a.Target,
				Directories: a.Directories,
				Command:     a.Command,
				Steps:       a.Steps,
				Notes:       a.Notes,
			}
			for _, p := range a.Pairs {
				ha.Pairs = append(ha.Pairs, htmlPair{
					Kept: p.Kept, KeptURL: fileURL(p.Kept),
					Redundant: p.Redundant, RedundantURL: fileURL(p.Redundant),
					Notes: p.Notes,
				})
			}
			ho.Actions = append(ho.Actions, ha)
		}
		doc.Opportunities = append(doc.Opportunities, ho)
	}

	return insightsTemplate.Execute(w, doc)
}

// fileURL is template.URL, not string: html/template only trusts
// http/https/mailto schemes by default and silently replaces
// anything else with "#ZgotmplZ". file:// is legitimate here because
// we construct it ourselves from an already-analyzed filesystem path
// via net/url, not from unescaped external input.
//
//nolint:gosec // G203: path comes from our own filesystem scan, not external/user-supplied input
func fileURL(path string) template.URL {
	return template.URL((&url.URL{Scheme: "file", Path: path}).String())
}

var insightsTemplate = template.Must(template.New("insights").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>DiskWise insights — {{.Root}}</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 15px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 1.5rem auto; max-width: 60rem; padding: 0 1rem; }
  h1 { font-size: 1.2rem; margin-bottom: 0.2rem; }
  .meta { color: #888; font-size: 0.85rem; margin-bottom: 1rem; }
  .summary { padding: 0.75rem 1rem; border-left: 3px solid #2f81f7; background: rgba(47,129,247,0.08); margin-bottom: 1.5rem; }
  table { border-collapse: collapse; width: 100%; font-size: 0.85rem; margin-bottom: 2rem; }
  th, td { text-align: left; padding: 0.4rem 0.6rem; border-bottom: 1px solid rgba(128,128,128,0.25); }
  th { white-space: nowrap; }
  .card { border: 1px solid rgba(128,128,128,0.3); border-radius: 0.5rem; padding: 1rem 1.25rem; margin-bottom: 1.25rem; }
  .card h2 { margin: 0 0 0.4rem; font-size: 1.05rem; }
  .badges { margin-bottom: 0.6rem; font-size: 0.8rem; color: #777; }
  .badge { display: inline-block; padding: 0.1rem 0.5rem; border-radius: 0.3rem; font-size: 0.75rem; font-weight: 600; margin-right: 0.3rem; }
  .cat-duplicate_backup, .cat-duplicate_download { background: #cf222e; color: #fff; }
  .cat-regeneratable_cache { background: #1a7f37; color: #fff; }
  .cat-managed_app_data { background: #9a6700; color: #fff; }
  .cat-unexplained_large, .cat-other { background: #57606a; color: #fff; }
  .conf-high { color: #1a7f37; font-weight: 600; }
  .conf-medium { color: #9a6700; font-weight: 600; }
  .conf-low { color: #cf222e; font-weight: 600; }
  ul { margin: 0.3rem 0; padding-left: 1.3rem; }
  .action { border-top: 1px dashed rgba(128,128,128,0.3); padding-top: 0.5rem; margin-top: 0.6rem; }
  .action-label { font-weight: 600; font-size: 0.85rem; }
  code { background: rgba(128,128,128,0.15); padding: 0.05rem 0.3rem; border-radius: 0.25rem; }
  a { color: inherit; }
</style>
</head>
<body>
<h1>DiskWise insights — {{.Root}}</h1>
<div class="meta">generated {{.GeneratedAt}}{{if .UsageLine}} · {{.UsageLine}}{{end}} · {{len .Opportunities}} opportunities</div>
<div class="summary">{{.Summary}}</div>

<table>
<thead><tr><th>#</th><th>Title</th><th>Category</th><th>Confidence</th><th>Est. reclaim</th></tr></thead>
<tbody>
{{range .Opportunities}}<tr>
  <td>{{.Rank}}</td>
  <td><a href="#opp-{{.Rank}}">{{.Title}}</a></td>
  <td><span class="badge cat-{{.Category}}">{{.Category}}</span></td>
  <td><span class="conf-{{.Confidence}}">{{.Confidence}}</span></td>
  <td>{{.EstimatedHuman}}</td>
</tr>{{end}}
</tbody>
</table>

{{range .Opportunities}}
<div class="card" id="opp-{{.Rank}}">
  <h2>{{.Rank}}. {{.Title}}</h2>
  <div class="badges">
    <span class="badge cat-{{.Category}}">{{.Category}}</span>
    <span class="conf-{{.Confidence}}">{{.Confidence}} confidence</span>
    · {{.EstimatedHuman}} estimated
  </div>
  <ul>
    {{$urls := .FileURLs}}
    {{range $i, $d := .Directories}}<li><a href="{{index $urls $i}}"><code>{{$d}}</code></a></li>{{end}}
  </ul>
  <p>{{.Description}}</p>
  {{if .Evidence}}<p><strong>Evidence:</strong></p><ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>{{end}}
  {{if .VerificationSteps}}<p><strong>Verify before acting:</strong></p><ol>{{range .VerificationSteps}}<li>{{.}}</li>{{end}}</ol>{{end}}
  {{range .Actions}}
  <div class="action">
    <div class="action-label">{{if .Target}}{{.Target}} — {{end}}{{.Method}}</div>
    {{if .Directories}}<ul>{{range .Directories}}<li><code>{{.}}</code></li>{{end}}</ul>{{end}}
    {{if .Pairs}}<table><thead><tr><th>Kept</th><th>Redundant</th><th>Notes</th></tr></thead><tbody>
    {{range .Pairs}}<tr><td><a href="{{.KeptURL}}"><code>{{.Kept}}</code></a></td><td><a href="{{.RedundantURL}}"><code>{{.Redundant}}</code></a></td><td>{{.Notes}}</td></tr>{{end}}
    </tbody></table>{{end}}
    {{if .Command}}<p><code>{{.Command}}</code></p>{{end}}
    {{if .Steps}}<ol>{{range .Steps}}<li>{{.}}</li>{{end}}</ol>{{end}}
    {{if .Notes}}<p>{{.Notes}}</p>{{end}}
  </div>
  {{end}}
</div>
{{end}}
</body>
</html>
`))
