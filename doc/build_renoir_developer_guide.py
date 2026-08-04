from html import escape
from pathlib import Path
import re

from reportlab.lib import colors
from reportlab.lib.pagesizes import LETTER
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import inch
from reportlab.platypus import Image, Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

ROOT = Path(__file__).resolve().parent
SOURCE = ROOT / "aspgen-renoir-developer-guide.md"
HTML = ROOT / "aspgen-renoir-developer-guide.html"
PDF = ROOT / "aspgen-renoir-developer-guide.pdf"
ASSETS = ROOT / "architecture-assets"


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
        paragraph = [line]
        i += 1
        while i < len(lines) and lines[i].strip() and not lines[i].startswith(("#", "|", "```", "- ")) and not re.match(r"^\d+\. ", lines[i]):
            paragraph.append(lines[i])
            i += 1
        result.append(("p", " ".join(paragraph)))
    return result


def inline(text):
    text = escape(text)
    text = re.sub(r"`([^`]+)`", r"<code>\1</code>", text)
    text = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", text)
    return text


def highlight(code):
    text = escape(code)
    text = re.sub(r"(\"[^\"]*\")", r'<span class="str">\1</span>', text)
    text = re.sub(r"(@\w+)", r'<span class="kw">\1</span>', text)
    text = re.sub(r"\b(aspgen|new|add|class|public|private|sealed|record|return|async|await|var|if|true|false|void|using|EditForm|InputText|InputNumber|InputCheckbox|DataAnnotationsValidator)\b", r'<span class="kw">\1</span>', text)
    return text


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
                slug = "s" + str(len(toc) + 1)
                toc.append((slug, title))
                content.append(f'<h{level} id="{slug}">{inline(title)}</h{level}>')
            else:
                if not first_h1:
                    content.append(f'<h1>{inline(title)}</h1>')
                first_h1 = False
        elif kind == "p":
            content.append(f"<p>{inline(block[1])}</p>")
        elif kind == "bullets":
            content.append("<ul>" + "".join(f"<li>{inline(x)}</li>" for x in block[1]) + "</ul>")
        elif kind == "numbers":
            content.append("<ol>" + "".join(f"<li>{inline(x)}</li>" for x in block[1]) + "</ol>")
        elif kind == "code":
            content.append(f'<pre class="code"><span class="lang">{escape(block[1])}</span>{highlight(block[2])}</pre>')
        elif kind == "table":
            rows = block[1]
            if rows:
                head = "".join(f"<th>{inline(x)}</th>" for x in rows[0])
                body = "".join("<tr>" + "".join(f"<td>{inline(x)}</td>" for x in row) + "</tr>" for row in rows[1:])
                content.append(f'<div class="table-wrap"><table><thead><tr>{head}</tr></thead><tbody>{body}</tbody></table></div>')
    toc_html = "".join(f'<li><a href="#{slug}">{inline(title)}</a></li>' for slug, title in toc)
    html = f'''<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>aspgen Renoir Developer Guide</title><style>
:root{{--navy:#0b2545;--blue:#2e74b5;--gold:#b7831e;--ink:#182230;--muted:#5b6573;--pale:#f4f6f9;--line:#c8d0d9}}*{{box-sizing:border-box}}body{{margin:0;background:#eef2f6;color:var(--ink);font:16px/1.58 Calibri,Arial,sans-serif}}.page{{max-width:1040px;margin:32px auto;background:#fff;padding:52px 76px 70px;box-shadow:0 12px 38px #0b25451f}}h1{{color:var(--navy);font-size:2.5rem;line-height:1.1;margin:0 0 22px}}h2{{color:var(--blue);font-size:1.5rem;margin:36px 0 10px;border-bottom:1px solid #e0e6ec;padding-bottom:5px}}h3{{color:#1f4d78;font-size:1.15rem;margin:24px 0 7px}}p{{margin:0 0 11px}}a{{color:var(--blue)}}ul,ol{{margin:0 0 14px;padding-left:28px}}li{{margin:0 0 5px}}code{{background:#eef2f6;padding:1px 4px;border-radius:3px;font-family:Consolas,monospace;font-size:.9em}}.code{{background:#101b2d;color:#dbe7f4;padding:18px 20px;border-radius:8px;overflow:auto;font:13px/1.55 Consolas,monospace;margin:12px 0 20px;box-shadow:0 7px 20px #0b25451a}}.code .lang{{display:block;color:#7fa9d2;font:700 10px/1.2 Arial;letter-spacing:.1em;text-transform:uppercase;margin-bottom:8px}}.kw{{color:#83d1ff}}.str{{color:#d8e99b}}.table-wrap{{overflow-x:auto;margin:14px 0 20px}}table{{border-collapse:collapse;width:100%;font-size:.91rem}}th{{background:var(--navy);color:#fff;text-align:left;padding:9px 10px}}td{{border:1px solid var(--line);padding:8px 10px;vertical-align:top}}tbody tr:nth-child(even) td{{background:var(--pale)}}.mast{{color:var(--gold);font-weight:700;letter-spacing:.12em;font-size:.8rem}}.toc{{background:var(--pale);border:1px solid #dfe5eb;padding:16px 22px;margin:15px 0 30px}}.toc h2{{font-size:1.1rem;margin:0 0 7px;border:0}}.toc ul{{columns:2;margin:0}}footer{{border-top:1px solid #dfe5eb;color:var(--muted);margin-top:44px;padding-top:14px;font-size:.85rem}}@media(max-width:720px){{.page{{margin:0;padding:28px 21px 45px}}.toc ul{{columns:1}}}}@media print{{body{{background:#fff}}.page{{box-shadow:none;margin:0;max-width:none;padding:0}}}}
</style></head><body><div class="page"><p class="mast">DEVELOPER GUIDE</p><h1>aspgen Renoir Developer Guide</h1><p>A practical, step-by-step guide to building and extending a Renoir-style DDD/Blazor application with aspgen.</p><nav class="toc"><h2>Contents</h2><ul>{toc_html}</ul></nav><main>{''.join(content)}</main><footer>Architecture Theta • Version 1.0 • Prepared 04 August 2026</footer></div></body></html>'''
    HTML.write_text(html, encoding="utf-8")


def pdf_output():
    styles = {
        "body": ParagraphStyle("body", fontName="Helvetica", fontSize=9.4, leading=12.3, spaceAfter=6, textColor=colors.HexColor("#182230")),
        "h1": ParagraphStyle("h1", fontName="Helvetica-Bold", fontSize=21, leading=25, spaceAfter=14, textColor=colors.HexColor("#0B2545")),
        "h2": ParagraphStyle("h2", fontName="Helvetica-Bold", fontSize=15, leading=18, spaceBefore=16, spaceAfter=7, textColor=colors.HexColor("#2E74B5")),
        "h3": ParagraphStyle("h3", fontName="Helvetica-Bold", fontSize=12, leading=15, spaceBefore=10, spaceAfter=5, textColor=colors.HexColor("#1F4D78")),
        "code": ParagraphStyle("code", fontName="Courier", fontSize=7.6, leading=9.5, leftIndent=8, rightIndent=8, spaceBefore=3, spaceAfter=8, backColor=colors.HexColor("#101B2D"), textColor=colors.HexColor("#D8E7F4")),
        "small": ParagraphStyle("small", fontName="Helvetica", fontSize=7.7, leading=9.5, textColor=colors.HexColor("#5B6573")),
    }
    story = [Paragraph("aspgen Renoir Developer Guide", styles["h1"]), Paragraph("A practical, step-by-step guide to building and extending a Renoir-style DDD/Blazor application with aspgen.", styles["body"]), Spacer(1, 10)]
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
        elif kind == "bullets":
            for item in block[1]:
                story.append(Paragraph("• " + inline(item), styles["body"]))
        elif kind == "numbers":
            for idx, item in enumerate(block[1], 1):
                story.append(Paragraph(f"{idx}. {inline(item)}", styles["body"]))
        elif kind == "code":
            story.append(Paragraph(escape(block[2]).replace("\n", "<br/>"), styles["code"]))
        elif kind == "table":
            rows = block[1]
            if rows:
                data = [[Paragraph(inline(x), styles["small"]) for x in row] for row in rows]
                widths = [6.5 * inch / len(rows[0])] * len(rows[0])
                table = Table(data, colWidths=widths, repeatRows=1)
                cmds = [("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#0B2545")), ("TEXTCOLOR", (0, 0), (-1, 0), colors.white), ("GRID", (0, 0), (-1, -1), .3, colors.HexColor("#C8D0D9")), ("VALIGN", (0, 0), (-1, -1), "TOP"), ("LEFTPADDING", (0, 0), (-1, -1), 6), ("RIGHTPADDING", (0, 0), (-1, -1), 6), ("TOPPADDING", (0, 0), (-1, -1), 5), ("BOTTOMPADDING", (0, 0), (-1, -1), 5)]
                for i in range(1, len(rows)):
                    cmds.append(("BACKGROUND", (0, i), (-1, i), colors.HexColor("#F4F6F9" if i % 2 else "#FFFFFF")))
                table.setStyle(TableStyle(cmds))
                story.extend([Spacer(1, 4), table, Spacer(1, 8)])
    def footer(canvas, doc):
        canvas.saveState(); canvas.setFont("Helvetica", 8); canvas.setFillColor(colors.HexColor("#5B6573")); canvas.drawRightString(7.5 * inch, .5 * inch, f"Architecture Theta  •  Page {doc.page}"); canvas.restoreState()
    SimpleDocTemplate(str(PDF), pagesize=LETTER, leftMargin=inch, rightMargin=inch, topMargin=.7 * inch, bottomMargin=.7 * inch, title="aspgen Renoir Developer Guide").build(story, onFirstPage=footer, onLaterPages=footer)


if __name__ == "__main__":
    html_output()
    pdf_output()
    print(HTML)
    print(PDF)
