from base64 import b64encode
from html import escape
from pathlib import Path

from docx import Document
from docx.table import Table
from docx.text.paragraph import Paragraph
from docx.oxml.ns import qn

ROOT = Path(__file__).resolve().parent
DOCX = ROOT / "aspgen-architecture-blueprint.docx"
HTML = ROOT / "aspgen-architecture-blueprint.html"
ASSETS = ROOT / "architecture-assets"


def iter_blocks(document):
    for child in document.element.body.iterchildren():
        if child.tag == qn("w:p"):
            yield Paragraph(child, document)
        elif child.tag == qn("w:tbl"):
            yield Table(child, document)


def data_uri(path):
    return "data:image/png;base64," + b64encode(path.read_bytes()).decode("ascii")


def text_of(paragraph):
    return "".join(run.text for run in paragraph.runs)


def render_table(table):
    rows = [[escape(" ".join(cell.text.split())) for cell in row.cells] for row in table.rows]
    if len(rows) == 1 and len(rows[0]) == 1:
        value = rows[0][0]
        label, _, detail = value.partition("  ")
        return f'<aside class="callout"><strong>{label}</strong> {detail}</aside>'
    header = "".join(f"<th>{cell}</th>" for cell in rows[0])
    body = "".join("<tr>" + "".join(f"<td>{cell}</td>" for cell in row) + "</tr>" for row in rows[1:])
    return f'<div class="table-wrap"><table><thead><tr>{header}</tr></thead><tbody>{body}</tbody></table></div>'


def build():
    document = Document(DOCX)
    parts, toc = [], []
    image_index = 0
    for block in iter_blocks(document):
        if isinstance(block, Table):
            parts.append(render_table(block))
            continue
        text = text_of(block).strip()
        if block._p.xpath(".//w:drawing"):
            image_index += 1
            image = ASSETS / ("gateway.png" if image_index == 1 else "flow.png")
            parts.append(f'<figure><img src="{data_uri(image)}" alt="Architecture diagram {image_index}"></figure>')
            continue
        if not text:
            continue
        style = block.style.name if block.style else "Normal"
        if style == "Heading 1":
            slug = f"section-{len(toc) + 1}"
            toc.append((slug, text))
            parts.append(f'<h2 id="{slug}">{escape(text)}</h2>')
        elif style == "Heading 2":
            parts.append(f'<h3>{escape(text)}</h3>')
        elif style == "Heading 3":
            parts.append(f'<h4>{escape(text)}</h4>')
        elif style == "List Bullet":
            parts.append(f'<p class="bullet">{escape(text)}</p>')
        elif style == "List Number":
            parts.append(f'<p class="number">{escape(text)}</p>')
        elif text.startswith("Figure "):
            parts.append(f'<figcaption>{escape(text)}</figcaption>')
        elif block._p.xpath(".//w:shd"):
            parts.append(f'<pre>{escape(text)}</pre>')
        elif text.startswith("aspgen ") or text.startswith("ARCHITECTURE"):
            parts.append(f'<p class="eyebrow">{escape(text)}</p>')
        else:
            parts.append(f'<p>{escape(text)}</p>')
    toc_html = "".join(f'<li><a href="#{slug}">{escape(text)}</a></li>' for slug, text in toc)
    html = f'''<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>aspgen Architecture Blueprint</title>
<style>
:root {{ --navy:#0b2545; --blue:#2e74b5; --gold:#b7831e; --ink:#182230; --muted:#5b6573; --pale:#f4f6f9; --line:#c8d0d9; }}
* {{ box-sizing:border-box; }} body {{ margin:0; background:#eef2f6; color:var(--ink); font:16px/1.58 Calibri,Arial,sans-serif; }}
.page {{ max-width:980px; margin:32px auto; background:#fff; padding:54px 78px 72px; box-shadow:0 12px 38px rgba(11,37,69,.12); }}
header {{ border-bottom:1px solid #dfe5eb; padding-bottom:28px; margin-bottom:34px; }} .eyebrow {{ color:var(--gold); font-weight:700; letter-spacing:.12em; font-size:.78rem; margin:0 0 8px; }}
header h1 {{ color:var(--navy); font-size:3.1rem; line-height:1; margin:0 0 8px; letter-spacing:-.04em; }} header .subtitle {{ color:var(--muted); font-size:1.3rem; margin:0 0 22px; }}
.meta {{ display:grid; grid-template-columns:1fr 1.6fr 1fr; border:1px solid var(--line); margin:22px 0; }} .meta div {{ padding:12px 14px; border-right:1px solid var(--line); }} .meta div:last-child {{ border-right:0; }}
.meta strong {{ display:block; color:var(--navy); font-size:.78rem; text-transform:uppercase; letter-spacing:.08em; }} .meta span {{ font-size:.9rem; }}
.callout {{ background:#e8eef5; border-left:5px solid var(--blue); padding:15px 18px; margin:18px 0 22px; }} .callout strong {{ color:var(--navy); letter-spacing:.06em; margin-right:7px; }}
h2 {{ color:var(--blue); font-size:1.55rem; margin:38px 0 11px; border-bottom:1px solid #e0e6ec; padding-bottom:5px; }} h3 {{ color:#1f4d78; font-size:1.17rem; margin:25px 0 8px; }} h4 {{ color:#1f4d78; font-size:1.05rem; margin:19px 0 6px; }} p {{ margin:0 0 10px; }}
.bullet {{ margin-left:22px; text-indent:-13px; margin-bottom:5px; }} .bullet:before {{ content:"•"; color:var(--blue); font-weight:700; margin-right:7px; }} .number {{ margin-left:27px; text-indent:-18px; margin-bottom:6px; counter-increment:step; }} .number:before {{ content:counter(step) "."; color:var(--blue); font-weight:700; margin-right:9px; }} main {{ counter-reset:step; }}
pre {{ background:#f2f4f7; border-left:4px solid #a8b5c3; padding:14px 16px; white-space:pre-wrap; font:13px/1.45 Consolas,monospace; color:#273444; overflow:auto; }} figure {{ margin:22px 0 4px; text-align:center; }} figure img {{ max-width:100%; height:auto; }} figcaption {{ color:var(--muted); font-size:.84rem; font-style:italic; text-align:center; margin:0 0 15px; }}
.table-wrap {{ overflow-x:auto; margin:14px 0 20px; }} table {{ border-collapse:collapse; width:100%; font-size:.9rem; }} th {{ background:var(--navy); color:#fff; text-align:left; padding:9px 10px; }} td {{ border:1px solid var(--line); padding:8px 10px; vertical-align:top; }} tbody tr:nth-child(even) td {{ background:var(--pale); }} a {{ color:var(--blue); }}
.toc {{ background:var(--pale); border:1px solid #dfe5eb; padding:18px 24px; margin:20px 0 32px; }} .toc h2 {{ margin:0 0 8px; border:0; font-size:1.1rem; }} .toc ul {{ margin:0; padding-left:22px; columns:2; }} footer {{ margin-top:45px; padding-top:14px; border-top:1px solid #dfe5eb; color:var(--muted); font-size:.84rem; }}
@media (max-width:720px) {{ .page {{ margin:0; padding:28px 22px 44px; }} header h1 {{ font-size:2.35rem; }} .meta {{ grid-template-columns:1fr; }} .meta div {{ border-right:0; border-bottom:1px solid var(--line); }} .meta div:last-child {{ border-bottom:0; }} .toc ul {{ columns:1; }} }} @media print {{ body {{ background:#fff; }} .page {{ box-shadow:none; margin:0; max-width:none; padding:0; }} a {{ color:inherit; text-decoration:none; }} }}
</style></head><body><div class="page"><header><p class="eyebrow">ARCHITECTURE BLUEPRINT</p><h1>aspgen</h1><p class="subtitle">Gatewayed generation for scalable .NET systems</p><div class="meta"><div><strong>Document</strong><span>Architecture Blueprint</span></div><div><strong>Audience</strong><span>Engineering teams and maintainers</span></div><div><strong>Status</strong><span>First release baseline</span></div></div></header>
<nav class="toc"><h2>Contents</h2><ul>{toc_html}</ul></nav><main>{''.join(parts)}</main><footer>Architecture Theta • Version 1.0 • Prepared 03 August 2026</footer></div></body></html>'''
    HTML.write_text(html, encoding="utf-8")
    print(HTML)


if __name__ == "__main__":
    build()
