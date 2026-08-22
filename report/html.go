package report

import (
	"fmt"
	"html/template"
	"io"
	"net/url"
	"time"

	"github.com/grokify/diskwise/policy"
)

// htmlRow is Row plus the presentation fields the template needs
// precomputed in Go rather than reimplemented in the template
// language (byte formatting, tier CSS class, file:// URI encoding).
type htmlRow struct {
	Row
	AllocatedHuman string
	LogicalHuman   string
	ConfidencePct  int
	TierClass      string
	// FileURL is template.URL, not string: html/template only trusts
	// http/https/mailto schemes by default and silently replaces
	// anything else with "#ZgotmplZ". file:// is legitimate here
	// because we construct it ourselves from an already-indexed
	// filesystem path via net/url, not from unescaped external input.
	FileURL   template.URL
	Scenarios []htmlScenario
}

type htmlScenario struct {
	Name             string
	Description      string
	ReclaimableHuman string
}

type htmlTierTotal struct {
	Tier  string
	Class string
	Total string
}

type htmlData struct {
	Root        string
	GeneratedAt string
	Rows        []htmlRow
	TierTotals  []htmlTierTotal
	GrandTotal  string
}

// WriteHTML renders rows as a single self-contained HTML file: an
// inline-styled, sortable/filterable table with no external script or
// stylesheet dependency, so it opens and works with a plain
// double-click and no network access. generatedAt is passed in
// (rather than computed here) because time.Now is not deterministic
// and callers may want a stable timestamp for testing or reproducible
// output.
func WriteHTML(w io.Writer, root string, rows []Row, generatedAt time.Time) error {
	data := htmlData{
		Root:        root,
		GeneratedAt: generatedAt.Format("2006-01-02 15:04:05 MST"),
	}

	totals := make(map[string]int64)
	var totalOrder []string
	var grandTotal int64

	for _, r := range rows {
		hr := htmlRow{
			Row:            r,
			AllocatedHuman: humanBytes(r.AllocatedSize),
			LogicalHuman:   humanBytes(r.LogicalSize),
			ConfidencePct:  int(r.Confidence*100 + 0.5),
			TierClass:      tierClass(r.Tier),
			FileURL:        fileURL(r.Path),
		}
		for _, sc := range r.Scenarios {
			hr.Scenarios = append(hr.Scenarios, htmlScenario{
				Name:             sc.Name,
				Description:      sc.Description,
				ReclaimableHuman: humanBytes(sc.ReclaimableBytes),
			})
		}
		data.Rows = append(data.Rows, hr)

		tier := string(r.Tier)
		if _, seen := totals[tier]; !seen {
			totalOrder = append(totalOrder, tier)
		}
		totals[tier] += r.AllocatedSize
		grandTotal += r.AllocatedSize
	}
	for _, tier := range totalOrder {
		data.TierTotals = append(data.TierTotals, htmlTierTotal{
			Tier:  tier,
			Class: tierClass(policy.ActionClass(tier)),
			Total: humanBytes(totals[tier]),
		})
	}
	data.GrandTotal = humanBytes(grandTotal)

	return htmlTemplate.Execute(w, data)
}

func fileURL(path string) template.URL {
	return template.URL((&url.URL{Scheme: "file", Path: path}).String())
}

// tierClass maps a tier to its CSS class name. Every policy.ActionClass
// value has a corresponding "tier-<value>" rule in the stylesheet, so
// this is a plain, injection-safe concatenation.
func tierClass(tier policy.ActionClass) string {
	return "tier-" + string(tier)
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

var htmlTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>DiskWise report — {{.Root}}</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 14px/1.4 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 1.5rem; }
  h1 { font-size: 1.1rem; margin: 0 0 0.2rem; }
  .meta { color: #888; font-size: 0.85rem; margin-bottom: 1rem; }
  .totals { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-bottom: 1rem; }
  .pill { border-radius: 999px; padding: 0.25rem 0.75rem; font-size: 0.8rem; border: 1px solid rgba(128,128,128,0.4); }
  #filter { width: 100%; max-width: 28rem; padding: 0.4rem 0.6rem; margin-bottom: 0.75rem;
            border: 1px solid rgba(128,128,128,0.4); border-radius: 0.4rem; font-size: 0.9rem; }
  table { border-collapse: collapse; width: 100%; font-size: 0.85rem; }
  th, td { text-align: left; padding: 0.4rem 0.6rem; border-bottom: 1px solid rgba(128,128,128,0.25); vertical-align: top; }
  th { cursor: pointer; user-select: none; white-space: nowrap; position: sticky; top: 0; background: Canvas; }
  th:hover { text-decoration: underline; }
  th.num, td.num { text-align: right; }
  td.path { word-break: break-all; max-width: 32rem; }
  td.reason { color: #777; max-width: 24rem; }
  .tier { display: inline-block; padding: 0.1rem 0.5rem; border-radius: 0.3rem; font-size: 0.75rem; font-weight: 600; }
  .tier-safe_delete { background: #1a7f37; color: #fff; }
  .tier-likely_safe { background: #2f81f7; color: #fff; }
  .tier-backup_then_delete { background: #9a6700; color: #fff; }
  .tier-review { background: #cf222e; color: #fff; }
  .tier-keep { background: #6e7781; color: #fff; }
  .tier-unknown { background: #57606a; color: #fff; }
  .scenarios { margin: 0.25rem 0 0; padding-left: 1.1rem; font-size: 0.8rem; color: #777; }
  a { color: inherit; }
</style>
</head>
<body>
<h1>DiskWise report — {{.Root}}</h1>
<div class="meta">generated {{.GeneratedAt}} · {{len .Rows}} findings · {{.GrandTotal}} total</div>
<div class="totals">
  {{range .TierTotals}}<span class="pill"><span class="tier {{.Class}}">{{.Tier}}</span> {{.Total}}</span>{{end}}
</div>
<input id="filter" type="search" placeholder="Filter by path, kind, or reason…" autofocus>
<table id="report">
<thead>
<tr>
  <th data-type="text">Tier</th>
  <th data-type="text">Kind</th>
  <th class="num" data-type="num">Allocated</th>
  <th class="num" data-type="num">Logical</th>
  <th class="num" data-type="num">Conf.</th>
  <th data-type="text">Path</th>
  <th data-type="text">Reason</th>
</tr>
</thead>
<tbody>
{{range .Rows}}
<tr>
  <td><span class="tier {{.TierClass}}">{{.Tier}}</span></td>
  <td>{{.Kind}}</td>
  <td class="num" data-sort="{{.AllocatedSize}}">{{.AllocatedHuman}}</td>
  <td class="num" data-sort="{{.LogicalSize}}">{{.LogicalHuman}}</td>
  <td class="num" data-sort="{{.Confidence}}">{{.ConfidencePct}}%</td>
  <td class="path"><a href="{{.FileURL}}">{{.Path}}</a>{{if gt .PathCount 1}} <span class="meta">({{.PathCount}} paths)</span>{{end}}</td>
  <td class="reason">{{.Reason}}
    {{if .Scenarios}}<ul class="scenarios">{{range .Scenarios}}<li>[{{.Name}}] {{.ReclaimableHuman}} — {{.Description}}</li>{{end}}</ul>{{end}}
  </td>
</tr>
{{end}}
</tbody>
</table>
<script>
(function () {
  var table = document.getElementById('report');
  var tbody = table.tBodies[0];
  var rows = Array.prototype.slice.call(tbody.rows);

  document.querySelectorAll('#report th').forEach(function (th, colIndex) {
    var asc = true;
    th.addEventListener('click', function () {
      var type = th.getAttribute('data-type');
      rows.sort(function (a, b) {
        var ca = a.cells[colIndex], cb = b.cells[colIndex];
        var va = type === 'num' ? parseFloat(ca.getAttribute('data-sort') || ca.textContent) : ca.textContent.trim().toLowerCase();
        var vb = type === 'num' ? parseFloat(cb.getAttribute('data-sort') || cb.textContent) : cb.textContent.trim().toLowerCase();
        if (va < vb) return asc ? -1 : 1;
        if (va > vb) return asc ? 1 : -1;
        return 0;
      });
      asc = !asc;
      rows.forEach(function (r) { tbody.appendChild(r); });
    });
  });

  var filter = document.getElementById('filter');
  filter.addEventListener('input', function () {
    var q = filter.value.trim().toLowerCase();
    rows.forEach(function (r) {
      r.style.display = !q || r.textContent.toLowerCase().indexOf(q) !== -1 ? '' : 'none';
    });
  });
})();
</script>
</body>
</html>
`))
