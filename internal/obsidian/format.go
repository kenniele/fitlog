package obsidian

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	markdownLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	wikiLink     = regexp.MustCompile(`!?\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	inlineCode   = regexp.MustCompile("`([^`]+)`")
	boldText     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicText   = regexp.MustCompile(`\*([^*]+)\*`)
)

func formatPage(article Article) string {
	return `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light dark">
<title>` + html.EscapeString(article.Title) + ` · fitlog</title>
<style>
:root{font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#202124;background:#f5f2ea}
body{margin:0;padding:24px 16px 64px}.page{max-width:760px;margin:auto;background:#fff;border:1px solid #e4ded1;border-radius:18px;padding:clamp(22px,5vw,52px);box-shadow:0 14px 40px rgba(60,50,30,.08)}
h1{font-size:clamp(2rem,6vw,3.4rem);line-height:1.06;margin:0 0 2rem}h2,h3,h4{line-height:1.25;margin:2em 0 .65em}p,li,blockquote{font-size:1.05rem;line-height:1.75}a{color:#32735f;text-decoration-thickness:.08em;text-underline-offset:.16em}
blockquote{margin:1.5em 0;padding:.2em 1.2em;border-left:4px solid #6c9d8e;background:#f1f7f4;border-radius:0 10px 10px 0}pre{overflow:auto;padding:1rem;border-radius:10px;background:#202522;color:#f4f4ef}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;background:#edf1ee;padding:.12em .35em;border-radius:5px}pre code{background:none;padding:0}
table{width:100%;border-collapse:collapse;font-size:.95rem}.table-wrap{overflow-x:auto;margin:1.5rem 0;border:1px solid #ddd6c8;border-radius:10px}th,td{padding:.7rem .8rem;text-align:left;vertical-align:top;border-bottom:1px solid #e7e1d6}th{background:#f2eee5;font-weight:700}tr:last-child td{border-bottom:0}
hr{border:0;border-top:1px solid #ddd6c8;margin:2.5rem 0}.source{margin-top:3rem;color:#777;font-size:.85rem}
@media(prefers-color-scheme:dark){:root{color:#e8e5dd;background:#151715}.page{background:#1d201e;border-color:#343934;box-shadow:none}a{color:#81c6ae}blockquote{background:#202a26}code{background:#2b312e}.table-wrap{border-color:#3b403c}th{background:#292d2a}th,td{border-color:#3b403c}.source{color:#aaa}}
</style>
</head>
<body><main class="page"><article><h1>` + html.EscapeString(article.Title) + `</h1>` + markdownBody(article.Markdown) + `</article><div class="source">Опубликовано из Obsidian через fitlog</div></main></body></html>`
}

func markdownBody(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var out strings.Builder
	var paragraph []string
	var code []string
	inCode := false
	list := ""

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		fmt.Fprintf(&out, "<p>%s</p>", inline(strings.Join(paragraph, " ")))
		paragraph = nil
	}
	closeList := func() {
		if list != "" {
			fmt.Fprintf(&out, "</%s>", list)
			list = ""
		}
	}

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			flushParagraph()
			closeList()
			if inCode {
				fmt.Fprintf(&out, "<pre><code>%s</code></pre>", html.EscapeString(strings.Join(code, "\n")))
				code = nil
			} else {
				code = nil
			}
			inCode = !inCode
			continue
		}
		if inCode {
			code = append(code, raw)
			continue
		}
		if line == "" {
			flushParagraph()
			closeList()
			continue
		}
		if i+1 < len(lines) && isTableHeader(line, strings.TrimSpace(lines[i+1])) {
			flushParagraph()
			closeList()
			headers := tableCells(line)
			out.WriteString(`<div class="table-wrap"><table><thead><tr>`)
			for _, cell := range headers {
				fmt.Fprintf(&out, "<th>%s</th>", inline(cell))
			}
			out.WriteString("</tr></thead><tbody>")
			i += 2 // skip the separator; i now points at the first data row
			for ; i < len(lines); i++ {
				row := strings.TrimSpace(lines[i])
				if !isTableRow(row) {
					i-- // let the outer loop process the first non-table line
					break
				}
				out.WriteString("<tr>")
				cells := tableCells(row)
				for column := range headers {
					value := ""
					if column < len(cells) {
						value = cells[column]
					}
					fmt.Fprintf(&out, "<td>%s</td>", inline(value))
				}
				out.WriteString("</tr>")
			}
			out.WriteString("</tbody></table></div>")
			continue
		}
		if line == "---" || line == "***" {
			flushParagraph()
			closeList()
			out.WriteString("<hr>")
			continue
		}
		if level, text, ok := heading(line); ok {
			flushParagraph()
			closeList()
			fmt.Fprintf(&out, "<h%d>%s</h%d>", level, inline(text), level)
			continue
		}
		if strings.HasPrefix(line, "> ") {
			flushParagraph()
			closeList()
			text := strings.TrimSpace(strings.TrimPrefix(line, "> "))
			text = strings.TrimPrefix(text, "[!NOTE]")
			text = strings.TrimPrefix(text, "[!TIP]")
			text = strings.TrimPrefix(text, "[!WARNING]")
			fmt.Fprintf(&out, "<blockquote>%s</blockquote>", inline(strings.TrimSpace(text)))
			continue
		}
		if text, ordered, ok := listItem(line); ok {
			flushParagraph()
			wanted := "ul"
			if ordered {
				wanted = "ol"
			}
			if list != wanted {
				closeList()
				list = wanted
				fmt.Fprintf(&out, "<%s>", list)
			}
			fmt.Fprintf(&out, "<li>%s</li>", inline(text))
			continue
		}
		closeList()
		paragraph = append(paragraph, line)
	}
	if inCode {
		fmt.Fprintf(&out, "<pre><code>%s</code></pre>", html.EscapeString(strings.Join(code, "\n")))
	}
	flushParagraph()
	closeList()
	return out.String()
}

func isTableHeader(header, separator string) bool {
	if !isTableRow(header) || !isTableRow(separator) {
		return false
	}
	headerCells, separatorCells := tableCells(header), tableCells(separator)
	if len(headerCells) == 0 || len(headerCells) != len(separatorCells) {
		return false
	}
	for _, cell := range separatorCells {
		cell = strings.TrimSpace(strings.Trim(cell, ":"))
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func isTableRow(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") && strings.Count(line, "|") >= 2
}

func tableCells(line string) []string {
	line = strings.TrimSpace(strings.TrimSpace(line))
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	raw := strings.Split(line, "|")
	cells := make([]string, len(raw))
	for i := range raw {
		cells[i] = strings.TrimSpace(raw[i])
	}
	return cells
}

func heading(line string) (int, string, bool) {
	for level := 1; level <= 6; level++ {
		prefix := strings.Repeat("#", level) + " "
		if strings.HasPrefix(line, prefix) {
			return level, strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return 0, "", false
}

func listItem(line string) (string, bool, bool) {
	for _, prefix := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), false, true
		}
	}
	if dot := strings.Index(line, ". "); dot > 0 {
		for _, r := range line[:dot] {
			if r < '0' || r > '9' {
				return "", false, false
			}
		}
		return strings.TrimSpace(line[dot+2:]), true, true
	}
	return "", false, false
}

func inline(text string) string {
	text = html.EscapeString(text)
	text = markdownLink.ReplaceAllStringFunc(text, func(match string) string {
		parts := markdownLink.FindStringSubmatch(match)
		href := html.UnescapeString(parts[2])
		parsed, err := url.Parse(href)
		if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
			return parts[1]
		}
		return `<a href="` + html.EscapeString(parsed.String()) + `" rel="noreferrer">` + parts[1] + `</a>`
	})
	text = wikiLink.ReplaceAllStringFunc(text, func(match string) string {
		parts := wikiLink.FindStringSubmatch(match)
		label := parts[2]
		if label == "" {
			label = parts[1]
		}
		return `<span>` + label + `</span>`
	})
	text = inlineCode.ReplaceAllString(text, `<code>$1</code>`)
	text = boldText.ReplaceAllString(text, `<strong>$1</strong>`)
	text = italicText.ReplaceAllString(text, `<em>$1</em>`)
	return text
}
