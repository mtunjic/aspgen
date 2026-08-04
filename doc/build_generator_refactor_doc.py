"""Builds the HTML and PDF renditions of aspgen-generator-refactor.md.

Mirrors the parser/theme approach used by build_generation_decision_guide.py.
Mermaid-fenced code blocks (```mermaid, first line `%% diagram: <id>`) render
as live Mermaid.js diagrams in the HTML output and as hand-drawn reportlab
diagrams (keyed by the same <id>) in the PDF output, since reportlab cannot
execute JavaScript.
"""

import math
from html import escape
from pathlib import Path
import re

from reportlab.lib import colors
from reportlab.lib.pagesizes import LETTER
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import inch
from reportlab.graphics.shapes import Drawing, Line, Polygon, Rect, String
from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

ROOT = Path(__file__).resolve().parent
SOURCE = ROOT / "aspgen-generator-refactor.md"
HTML = ROOT / "aspgen-generator-refactor.html"
PDF = ROOT / "aspgen-generator-refactor.pdf"

NAVY = colors.HexColor("#0B2545")
BLUE = colors.HexColor("#2E74B5")
GOLD = colors.HexColor("#B7831E")
PALE = colors.HexColor("#F4F6F9")
LINE = colors.HexColor("#C8D0D9")
MUTED = colors.HexColor("#5B6573")


def blocks():
    lines = SOURCE.read_text(encoding="utf-8").splitlines()
    result, i = [], 0
    while i < len(lines):
        line = lines[i]
        if not line.strip():
            i += 1
            continue
        if line.startswith("```"):
            language = line[3:].strip()
            i += 1
            code = []
            while i < len(lines) and not lines[i].startswith("```"):
                code.append(lines[i])
                i += 1
            i += 1
            result.append(("code", language, "\n".join(code)))
            continue
        if line.startswith("# "):
            result.append(("h1", line[2:].strip()))
            i += 1
            continue
        if line.startswith("## "):
            result.append(("h2", line[3:].strip()))
            i += 1
            continue
        if line.startswith("### "):
            result.append(("h3", line[4:].strip()))
            i += 1
            continue
        if line.startswith(">"):
            note = []
            while i < len(lines) and lines[i].startswith(">"):
                note.append(lines[i].lstrip(">").strip())
                i += 1
            result.append(("note", " ".join(note)))
            continue
        if line.startswith("|"):
            rows = []
            while i < len(lines) and lines[i].startswith("|"):
                cells = [c.strip() for c in lines[i].strip().strip("|").split("|")]
                if not all(set(c) <= {"-", ":", " "} for c in cells):
                    rows.append(cells)
                i += 1
            result.append(("table", rows))
            continue
        if line.startswith("- "):
            items = []
            while i < len(lines) and lines[i].startswith("- "):
                items.append(lines[i][2:].strip())
                i += 1
            result.append(("bullets", items))
            continue
        if re.match(r"^\d+\. ", line):
            items = []
            while i < len(lines) and re.match(r"^\d+\. ", lines[i]):
                items.append(re.sub(r"^\d+\. ", "", lines[i]).strip())
                i += 1
            result.append(("numbers", items))
            continue
        if line.strip() == "---":
            result.append(("hr",))
            i += 1
            continue
        paragraph = [line]
        i += 1
        while i < len(lines) and lines[i].strip() and not lines[i].startswith(("#", "|", "```", "- ", ">")) and not re.match(r"^\d+\. ", lines[i]) and lines[i].strip() != "---":
            paragraph.append(lines[i])
            i += 1
        result.append(("p", " ".join(paragraph)))
    return result


def _link(match):
    label, target = match.group(1), match.group(2)
    if target.startswith("#"):
        return label  # in-page anchors aren't wired up in the PDF output
    return f'<a href="{target}">{label}</a>'


def inline(text):
    text = escape(text)
    text = re.sub(r"`([^`]+)`", r"<code>\1</code>", text)
    text = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", text)
    text = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", _link, text)
    return text


def highlight(code):
    text = escape(code)
    text = re.sub(r"(&#34;[^&]*?&#34;)", r'<span class="str">\1</span>', text)
    text = re.sub(r"\b(func|type|struct|map|return|if|else|nil|var|const|package|import|error|string|bool|int)\b", r'<span class="kw">\1</span>', text)
    return text


def diagram_id(code):
    match = re.match(r"^\s*%%\s*diagram:\s*(\S+)", code)
    return match.group(1) if match else None


# ---------------------------------------------------------------------------
# PDF diagrams (hand-drawn, since reportlab cannot run Mermaid/JS)
# ---------------------------------------------------------------------------

def _box(d, x, y, w, h, lines, fill=BLUE, text_color=colors.white, font_size=7.2):
    d.add(Rect(x, y, w, h, fillColor=fill, strokeColor=NAVY, strokeWidth=0.75, rx=5, ry=5))
    line_h = font_size + 2.6
    total_h = len(lines) * line_h
    start_y = y + h / 2 + total_h / 2 - line_h + font_size * 0.32
    for i, text in enumerate(lines):
        font = "Helvetica-Bold" if i == 0 else "Helvetica"
        d.add(String(x + w / 2, start_y - i * line_h, text, fontName=font, fontSize=font_size, fillColor=text_color, textAnchor="middle"))


def _arrow(d, x1, y1, x2, y2, color=MUTED, width=1.1):
    d.add(Line(x1, y1, x2, y2, strokeColor=color, strokeWidth=width))
    angle = math.atan2(y2 - y1, x2 - x1)
    size = 5.5
    p1 = (x2 - size * math.cos(angle - 0.42), y2 - size * math.sin(angle - 0.42))
    p2 = (x2 - size * math.cos(angle + 0.42), y2 - size * math.sin(angle + 0.42))
    d.add(Polygon([x2, y2, p1[0], p1[1], p2[0], p2[1]], fillColor=color, strokeColor=color))


def draw_package_split():
    d = Drawing(460, 300)
    _box(d, 130, 262, 200, 30, ["Run(args)", "generator.go"], fill=NAVY)
    _box(d, 5, 205, 140, 32, ["newProject()", "new_project.go"], fill=BLUE)
    _box(d, 160, 205, 140, 32, ["add()", "add.go + addHandlers"], fill=GOLD)
    _box(d, 315, 205, 140, 32, ["templateCommand()", "render.go"], fill=BLUE)
    for x in (75, 230, 385):
        _arrow(d, 230, 262, x, 237)
    handlers = [
        (5, ["add_ddd.go", "context · aggregate", "value-object · repository", "domain-service · event"]),
        (120, ["add_webapi.go", "feature · ui", "database · service"]),
        (235, ["add_entity.go", "entity"]),
        (350, ["add_module.go", "module"]),
    ]
    for x, lines in handlers:
        _box(d, x, 130, 105, 55, lines, fill=BLUE, font_size=6.6)
        _arrow(d, 230, 205, x + 52, 185)
    d.add(Rect(5, 20, 450, 75, fillColor=PALE, strokeColor=NAVY, strokeWidth=0.75, rx=5, ry=5))
    d.add(String(230, 78, "Shared foundation (unchanged behavior, now isolated)", fontName="Helvetica-Bold", fontSize=7.6, fillColor=NAVY, textAnchor="middle"))
    d.add(String(230, 62, "flags.go · manifest.go · types.go · render.go", fontName="Helvetica", fontSize=7.2, fillColor=NAVY, textAnchor="middle"))
    d.add(String(230, 48, "project_files.go · solution.go · seed.go · project_markers.go", fontName="Helvetica", fontSize=7.2, fillColor=NAVY, textAnchor="middle"))
    for x in (75, 172, 287, 402, 430):
        _arrow(d, x, 130 if x != 75 else 205, x, 95, color=LINE)
    return d


def draw_add_flow():
    d = Drawing(460, 130)
    steps = [
        (5, ["CLI args", "aspgen add ..."]),
        (98, ["add() prologue", "load manifest, resolve", "theme / backend"]),
        (191, ["addHandlers", "[component]"]),
        (284, ["Handler", "e.g. addEntityCmd"]),
        (377, ["manifest.json", "+ generated files"]),
    ]
    colors_cycle = [NAVY, BLUE, GOLD, BLUE, NAVY]
    for (x, lines), fill in zip(steps, colors_cycle):
        _box(d, x, 55, 80, 45, lines, fill=fill, font_size=6.4)
    for i in range(len(steps) - 1):
        x1 = steps[i][0] + 80
        x2 = steps[i + 1][0]
        _arrow(d, x1, 77, x2, 77)
    return d


def draw_extension_points():
    d = Drawing(460, 190)
    top = [
        (5, "addEntityCmd"),
        (122, "addAggregateCmd"),
        (239, "addFeatureCmd"),
        (356, "... 12 handlers total"),
    ]
    for x, label in top:
        _box(d, x, 140, 100, 32, [label], fill=BLUE, font_size=6.8)
    bottom = [
        (5, ["types.go", "csharpTypes registry"]),
        (170, ["project_markers.go", "readMarkerFile / writeMarkerFile"]),
        (335, ["manifest.go", "isNonSimpleWebAPI / isWPFProject", "isLocalDDDWpf"]),
    ]
    for x, lines in bottom:
        _box(d, x, 20, 120, 50, lines, fill=GOLD, font_size=6.4)
    edges = [(0, 0), (0, 2), (1, 0), (1, 1), (1, 2), (2, 1), (2, 2)]
    for ti, bi in edges:
        tx = top[ti][0] + 50
        bx = bottom[bi][0] + 60
        _arrow(d, tx, 140, bx, 70, color=LINE)
    return d


DIAGRAMS = {
    "package-split": draw_package_split,
    "add-flow": draw_add_flow,
    "extension-points": draw_extension_points,
}


# ---------------------------------------------------------------------------
# HTML
# ---------------------------------------------------------------------------

def html_output():
    content = []
    toc = []
    first_h1 = True
    for block in blocks():
        kind = block[0]
        if kind in {"h1", "h2", "h3"}:
            level = int(kind[-1]) + 1
            title = block[1]
            if kind != "h1":
                slug = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
                toc.append((slug, title))
                content.append(f'<h{level} id="{slug}">{inline(title)}</h{level}>')
            else:
                if not first_h1:
                    content.append(f'<h1>{inline(title)}</h1>')
                first_h1 = False
        elif kind == "p":
            content.append(f"<p>{inline(block[1])}</p>")
        elif kind == "note":
            content.append(f'<aside class="callout">{inline(block[1])}</aside>')
        elif kind == "bullets":
            content.append("<ul>" + "".join(f"<li>{inline(x)}</li>" for x in block[1]) + "</ul>")
        elif kind == "numbers":
            content.append("<ol>" + "".join(f"<li>{inline(x)}</li>" for x in block[1]) + "</ol>")
        elif kind == "hr":
            content.append("<hr>")
        elif kind == "code":
            language, code = block[1], block[2]
            if language == "mermaid":
                did = diagram_id(code) or "diagram"
                content.append(f'<figure class="diagram"><div class="mermaid" id="{did}">{escape(code)}</div></figure>')
            else:
                content.append(f'<pre class="code"><span class="lang">{escape(language)}</span>{highlight(code)}</pre>')
        elif kind == "table":
            rows = block[1]
            if rows:
                head = "".join(f"<th>{inline(x)}</th>" for x in rows[0])
                body = "".join("<tr>" + "".join(f"<td>{inline(x)}</td>" for x in row) + "</tr>" for row in rows[1:])
                content.append(f'<div class="table-wrap"><table><thead><tr>{head}</tr></thead><tbody>{body}</tbody></table></div>')
    toc_html = "".join(f'<li><a href="#{slug}">{inline(title)}</a></li>' for slug, title in toc)
    html = f'''<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>aspgen Generator Refactor</title><style>
:root{{--navy:#0b2545;--blue:#2e74b5;--gold:#b7831e;--ink:#182230;--muted:#5b6573;--pale:#f4f6f9;--line:#c8d0d9}}*{{box-sizing:border-box}}body{{margin:0;background:#eef2f6;color:var(--ink);font:16px/1.58 Calibri,Arial,sans-serif}}.page{{max-width:1040px;margin:32px auto;background:#fff;padding:52px 76px 70px;box-shadow:0 12px 38px #0b25451f}}h1{{color:var(--navy);font-size:2.5rem;line-height:1.1;margin:0 0 22px}}h2{{color:var(--blue);font-size:1.5rem;margin:36px 0 10px;border-bottom:1px solid #e0e6ec;padding-bottom:5px}}h3{{color:#1f4d78;font-size:1.15rem;margin:24px 0 7px}}p{{margin:0 0 11px}}a{{color:var(--blue)}}ul,ol{{margin:0 0 14px;padding-left:28px}}li{{margin:0 0 5px}}hr{{border:0;border-top:1px solid var(--line);margin:28px 0}}code{{background:#eef2f6;padding:1px 4px;border-radius:3px;font-family:Consolas,monospace;font-size:.9em}}.code{{background:#101b2d;color:#dbe7f4;padding:18px 20px;border-radius:8px;overflow:auto;font:13px/1.55 Consolas,monospace;margin:12px 0 20px;box-shadow:0 7px 20px #0b25451a}}.code .lang{{display:block;color:#7fa9d2;font:700 10px/1.2 Arial;letter-spacing:.1em;text-transform:uppercase;margin-bottom:8px}}.kw{{color:#83d1ff}}.str{{color:#d8e99b}}.table-wrap{{overflow-x:auto;margin:14px 0 20px}}table{{border-collapse:collapse;width:100%;font-size:.91rem}}th{{background:var(--navy);color:#fff;text-align:left;padding:9px 10px}}td{{border:1px solid var(--line);padding:8px 10px;vertical-align:top}}tbody tr:nth-child(even) td{{background:var(--pale)}}.mast{{color:var(--gold);font-weight:700;letter-spacing:.12em;font-size:.8rem}}.toc{{background:var(--pale);border:1px solid #dfe5eb;padding:16px 22px;margin:15px 0 30px}}.toc h2{{font-size:1.1rem;margin:0 0 7px;border:0}}.toc ul{{columns:2;margin:0}}.callout{{background:#fff7e8;border:1px solid #ecd9ac;border-left:4px solid var(--gold);padding:12px 16px;margin:14px 0 20px;border-radius:4px;font-size:.94rem}}figure.diagram{{margin:18px 0 26px;padding:18px;background:var(--pale);border:1px solid #dfe5eb;border-radius:8px;text-align:center;overflow:auto}}.mermaid{{display:inline-block}}footer{{border-top:1px solid #dfe5eb;color:var(--muted);margin-top:44px;padding-top:14px;font-size:.85rem}}@media(max-width:720px){{.page{{margin:0;padding:28px 21px 45px}}.toc ul{{columns:1}}}}@media print{{body{{background:#fff}}.page{{box-shadow:none;margin:0;max-width:none;padding:0}}}}
</style></head><body><div class="page"><p class="mast">GENERATOR ARCHITECTURE</p><h1>aspgen Generator Refactor</h1><p>Why <code>internal/generator</code> was split into cohesive files, how the pieces fit together, and how to extend it safely.</p><nav class="toc"><h2>Contents</h2><ul>{toc_html}</ul></nav><main>{''.join(content)}</main><footer>aspgen • refactor branch • Prepared 04 August 2026</footer></div></body></html>
<script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
<script>mermaid.initialize({{startOnLoad:true,theme:'base',themeVariables:{{primaryColor:'#2e74b5',primaryTextColor:'#ffffff',primaryBorderColor:'#0b2545',lineColor:'#5b6573',secondaryColor:'#b7831e',tertiaryColor:'#f4f6f9',actorBkg:'#2e74b5',actorTextColor:'#ffffff',actorBorder:'#0b2545',actorLineColor:'#5b6573',signalColor:'#182230',signalTextColor:'#182230',labelBoxBkgColor:'#f4f6f9',labelBoxBorderColor:'#c8d0d9',labelTextColor:'#182230',loopTextColor:'#182230',noteBkgColor:'#fff7e8',noteTextColor:'#182230',noteBorderColor:'#b7831e',sequenceNumberColor:'#ffffff'}}}});</script>
'''
    HTML.write_text(html, encoding="utf-8")


# ---------------------------------------------------------------------------
# PDF
# ---------------------------------------------------------------------------

def pdf_output():
    styles = {
        "body": ParagraphStyle("body", fontName="Helvetica", fontSize=9.4, leading=12.3, spaceAfter=6, textColor=colors.HexColor("#182230")),
        "h1": ParagraphStyle("h1", fontName="Helvetica-Bold", fontSize=21, leading=25, spaceAfter=14, textColor=NAVY),
        "h2": ParagraphStyle("h2", fontName="Helvetica-Bold", fontSize=15, leading=18, spaceBefore=16, spaceAfter=7, textColor=BLUE),
        "h3": ParagraphStyle("h3", fontName="Helvetica-Bold", fontSize=12, leading=15, spaceBefore=10, spaceAfter=5, textColor=colors.HexColor("#1F4D78")),
        "code": ParagraphStyle("code", fontName="Courier", fontSize=7.6, leading=9.5, leftIndent=8, rightIndent=8, spaceBefore=3, spaceAfter=8, backColor=colors.HexColor("#101B2D"), textColor=colors.HexColor("#D8E7F4")),
        "small": ParagraphStyle("small", fontName="Helvetica", fontSize=7.7, leading=9.5, textColor=MUTED),
        "note": ParagraphStyle("note", fontName="Helvetica-Oblique", fontSize=9, leading=12, spaceBefore=4, spaceAfter=10, textColor=colors.HexColor("#5A4207"), backColor=colors.HexColor("#FFF7E8"), borderPadding=8, borderColor=GOLD, borderWidth=0.75),
        "caption": ParagraphStyle("caption", fontName="Helvetica-Oblique", fontSize=7.6, leading=9.5, spaceBefore=4, spaceAfter=12, textColor=MUTED, alignment=1),
    }
    story = [
        Paragraph("aspgen Generator Refactor", styles["h1"]),
        Paragraph("Why internal/generator was split into cohesive files, how the pieces fit together, and how to extend it safely.", styles["body"]),
        Spacer(1, 10),
    ]
    first_h1 = True
    for block in blocks():
        kind = block[0]
        if kind == "h1":
            if not first_h1:
                story.append(Paragraph(inline(block[1]), styles["h1"]))
            first_h1 = False
        elif kind == "h2":
            story.append(Paragraph(inline(block[1]), styles["h2"]))
        elif kind == "h3":
            story.append(Paragraph(inline(block[1]), styles["h3"]))
        elif kind == "p":
            story.append(Paragraph(inline(block[1]), styles["body"]))
        elif kind == "note":
            story.append(Paragraph("Note: " + inline(block[1]), styles["note"]))
        elif kind == "bullets":
            for item in block[1]:
                story.append(Paragraph("• " + inline(item), styles["body"]))
        elif kind == "numbers":
            for idx, item in enumerate(block[1], 1):
                story.append(Paragraph(f"{idx}. {inline(item)}", styles["body"]))
        elif kind == "hr":
            story.append(Spacer(1, 4))
        elif kind == "code":
            language, code = block[1], block[2]
            if language == "mermaid":
                did = diagram_id(code)
                factory = DIAGRAMS.get(did)
                if factory:
                    story.append(Spacer(1, 4))
                    story.append(factory())
                    story.append(Paragraph(f"Figure: {did.replace('-', ' ')}", styles["caption"]))
            else:
                story.append(Paragraph(escape(code).replace("\n", "<br/>"), styles["code"]))
        elif kind == "table":
            rows = block[1]
            if rows:
                data = [[Paragraph(inline(x), styles["small"]) for x in row] for row in rows]
                widths = [6.5 * inch / len(rows[0])] * len(rows[0])
                table = Table(data, colWidths=widths, repeatRows=1)
                cmds = [("BACKGROUND", (0, 0), (-1, 0), NAVY), ("TEXTCOLOR", (0, 0), (-1, 0), colors.white), ("GRID", (0, 0), (-1, -1), .3, LINE), ("VALIGN", (0, 0), (-1, -1), "TOP"), ("LEFTPADDING", (0, 0), (-1, -1), 6), ("RIGHTPADDING", (0, 0), (-1, -1), 6), ("TOPPADDING", (0, 0), (-1, -1), 5), ("BOTTOMPADDING", (0, 0), (-1, -1), 5)]
                for i in range(1, len(rows)):
                    cmds.append(("BACKGROUND", (0, i), (-1, i), colors.HexColor("#F4F6F9" if i % 2 else "#FFFFFF")))
                table.setStyle(TableStyle(cmds))
                story.extend([Spacer(1, 4), table, Spacer(1, 8)])

    def footer(canvas, doc):
        canvas.saveState()
        canvas.setFont("Helvetica", 8)
        canvas.setFillColor(MUTED)
        canvas.drawRightString(7.5 * inch, .5 * inch, f"aspgen Generator Refactor  •  Page {doc.page}")
        canvas.restoreState()

    SimpleDocTemplate(str(PDF), pagesize=LETTER, leftMargin=inch, rightMargin=inch, topMargin=.7 * inch, bottomMargin=.7 * inch, title="aspgen Generator Refactor").build(story, onFirstPage=footer, onLaterPages=footer)


if __name__ == "__main__":
    html_output()
    pdf_output()
    print(HTML)
    print(PDF)
