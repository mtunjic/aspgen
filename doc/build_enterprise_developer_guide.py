"""Render doc/aspgen-enterprise-developer-guide.md to synchronized HTML + PDF
using the aspgen Document Theme (doc/aspgen-document-theme.md).

Usage:
    python doc/build_enterprise_developer_guide.py

Requires: Markdown, weasyprint (both already installed on this machine).
Mermaid fenced code blocks are rendered as plain preformatted text (dark
code-block style) since there is no offline mermaid renderer available --
they remain readable as ASCII-art-style diagrams either way.
"""
import re
import subprocess
import shutil
from pathlib import Path

import markdown as md

DOC_DIR = Path(__file__).resolve().parent
SRC = DOC_DIR / "aspgen-enterprise-developer-guide.md"
OUT_HTML = DOC_DIR / "aspgen-enterprise-developer-guide.html"
OUT_PDF = DOC_DIR / "aspgen-enterprise-developer-guide.pdf"

CSS = """
:root { --navy:#0b2545; --blue:#2e74b5; --darkblue:#1f4d78; --gold:#b7831e;
        --ink:#182230; --muted:#5b6573; --pale:#e8eef5; --light:#f4f6f9; --line:#c8d0d9; }
* { box-sizing:border-box; }
body { margin:0; background:#eef2f6; color:var(--ink); font:16px/1.58 Calibri,Arial,sans-serif; }
.page { max-width:980px; margin:32px auto; background:#fff; padding:54px 78px 72px; box-shadow:0 12px 38px rgba(11,37,69,.12); }
header { border-bottom:1px solid #dfe5eb; padding-bottom:28px; margin-bottom:34px; }
.eyebrow { color:var(--gold); font-weight:700; letter-spacing:.12em; font-size:.78rem; margin:0 0 8px; text-transform:uppercase; }
header h1 { color:var(--navy); font-size:2.7rem; line-height:1.05; margin:0 0 10px; letter-spacing:-.02em; }
header .subtitle { color:var(--muted); font-size:1.15rem; margin:0 0 22px; }
.meta { display:grid; grid-template-columns:repeat(4,1fr); border:1px solid var(--line); margin:22px 0; }
.meta div { padding:12px 14px; border-right:1px solid var(--line); }
.meta div:last-child { border-right:0; }
.meta strong { display:block; color:var(--navy); font-size:.72rem; text-transform:uppercase; letter-spacing:.08em; }
.meta span { font-size:.85rem; }
.callout, blockquote { background:var(--pale); border-left:5px solid var(--blue); padding:15px 18px; margin:18px 0 22px; }
blockquote { border-radius:0 6px 6px 0; }
blockquote p { margin:0; }
h1 { color:var(--navy); }
h2 { color:var(--blue); font-size:1.5rem; margin:38px 0 11px; border-bottom:1px solid #e0e6ec; padding-bottom:5px; }
h3 { color:var(--darkblue); font-size:1.15rem; margin:25px 0 8px; }
h4 { color:var(--darkblue); font-size:1.02rem; margin:18px 0 6px; }
p { margin:0 0 10px; }
ul, ol { margin:0 0 14px; padding-left:24px; }
li { margin-bottom:4px; }
pre, code { font:13px/1.5 Consolas,"Courier New",monospace; }
p code, li code, td code, h1 code, h2 code, h3 code { background:var(--light); color:var(--darkblue); padding:1px 5px; border-radius:4px; }
pre { background:var(--navy); color:#dce6f0; padding:14px 18px; border-radius:8px; overflow-x:auto; margin:14px 0 20px; }
pre code { background:none; color:inherit; padding:0; border-radius:0; }
.table-wrap { overflow-x:auto; margin:14px 0 20px; }
table { border-collapse:collapse; width:100%; font-size:.86rem; }
th { background:var(--navy); color:#fff; text-align:left; padding:9px 10px; }
td { border:1px solid var(--line); padding:8px 10px; vertical-align:top; }
tbody tr:nth-child(even) td { background:var(--light); }
a { color:var(--blue); }
hr { border:0; border-top:1px solid #dfe5eb; margin:26px 0; }
.toc { background:var(--light); border:1px solid #dfe5eb; padding:18px 24px; margin:20px 0 32px; }
.toc h2 { margin:0 0 8px; border:0; font-size:1.05rem; }
.toc ol { margin:0; padding-left:22px; columns:2; column-gap:32px; }
footer.doc-footer { margin-top:45px; padding-top:14px; border-top:1px solid #dfe5eb; color:var(--muted); font-size:.84rem; }
@media print {
  body { background:#fff; }
  .page { box-shadow:none; margin:0; max-width:none; padding:0; }
  a { color:inherit; text-decoration:none; }
  pre { padding:10px 14px; }
}
@page { size: Letter; margin: 1in;
  @bottom-center { content: "aspgen Enterprise Developer Guide  \\2022  page " counter(page); color:#5b6573; font-size:8.5pt; font-family:Calibri,Arial,sans-serif; }
}
"""


def parse_masthead(text: str):
    body_start = text.index("\n## Contents")
    head, body = text[:body_start], text[body_start + 1:]
    lines = head.splitlines()
    kicker = lines[0].strip()
    title = next(l[2:].strip() for l in lines if l.startswith("# "))
    subtitle = next(l.strip() for l in lines if l and not l.startswith(("#", "DEVELOPER")) and not l.startswith("**Document:**"))
    meta_line = next(l.strip() for l in lines if l.startswith("**Document:**"))
    meta_pairs = re.findall(r"\*\*([^*]+):\*\*\s*([^\u00b7]+?)(?:\s*\u00b7\s*|$)", meta_line)
    # Only the first (masthead) blockquote block, not later in-body callouts.
    callout_lines = []
    in_quote = False
    for l in lines:
        if l.startswith("> "):
            callout_lines.append(l[2:].strip())
            in_quote = True
        elif in_quote:
            break
    callout = " ".join(callout_lines)
    return kicker, title, subtitle, meta_pairs, callout, body


def render_meta(meta_pairs):
    cells = []
    for key, value in meta_pairs:
        value_html = md.markdown(value.strip())
        value_html = re.sub(r"^<p>|</p>$", "", value_html.strip())
        cells.append(f"<div><strong>{key.strip()}</strong><span>{value_html}</span></div>")
    return "".join(cells)


def wrap_tables(html: str) -> str:
    return re.sub(r"(<table>.*?</table>)", r'<div class="table-wrap">\1</div>', html, flags=re.S)


def main():
    text = SRC.read_text(encoding="utf-8")
    kicker, title, subtitle, meta_pairs, callout, body = parse_masthead(text)
    callout_html = md.markdown(callout.replace("**Decision in one paragraph.**", "DECISION")).strip()
    callout_html = re.sub(r"^<p>DECISION", "<p><strong>DECISION</strong>", callout_html)

    body_html = md.markdown(body, extensions=["fenced_code", "tables", "sane_lists", "toc"])
    body_html = wrap_tables(body_html)
    # Render "## Contents" as a styled TOC box instead of a plain heading + list.
    body_html = body_html.replace('<h2 id="contents">Contents</h2>', "", 1)
    body_html = re.sub(
        r"(<ol>.*?</ol>)",
        r'<nav class="toc"><h2>Contents</h2>\1</nav>',
        body_html,
        count=1,
        flags=re.S,
    )

    meta_html = render_meta(meta_pairs)

    html_doc = f"""<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<style>{CSS}</style></head><body><div class="page">
<header>
  <p class="eyebrow">{kicker}</p>
  <h1>{title}</h1>
  <p class="subtitle">{subtitle}</p>
  <div class="meta">{meta_html}</div>
  <aside class="callout">{callout_html}</aside>
</header>
<main>{body_html}</main>
<footer class="doc-footer">aspgen &middot; Enterprise Developer Guide &middot; generated from aspgen-enterprise-developer-guide.md</footer>
</div></body></html>"""

    OUT_HTML.write_text(html_doc, encoding="utf-8")
    print(f"Wrote {OUT_HTML}")
    write_pdf()


def write_pdf():
    """Try WeasyPrint first (needs native GTK/Pango libs); fall back to a
    headless Chromium-based browser's built-in print-to-pdf, which needs no
    extra native dependencies on Windows."""
    try:
        from weasyprint import HTML
        HTML(string=OUT_HTML.read_text(encoding="utf-8"), base_url=str(DOC_DIR)).write_pdf(str(OUT_PDF))
        print(f"Wrote {OUT_PDF} (weasyprint)")
        return
    except Exception as exc:  # noqa: BLE001 - deliberately broad, falling back below
        print(f"weasyprint unavailable ({exc.__class__.__name__}), falling back to headless Edge/Chrome")

    browser = shutil.which("msedge") or shutil.which("chrome") or shutil.which("google-chrome")
    for candidate in (
        r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
        r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
        r"C:\Program Files\Google\Chrome\Application\chrome.exe",
    ):
        if browser:
            break
        if Path(candidate).exists():
            browser = candidate
    if not browser:
        raise RuntimeError("No PDF backend available: install weasyprint's native deps or a Chromium-based browser.")

    file_url = OUT_HTML.resolve().as_uri()
    subprocess.run(
        [browser, "--headless", "--disable-gpu", "--no-pdf-header-footer",
         f"--print-to-pdf={OUT_PDF}", file_url],
        check=True,
    )
    print(f"Wrote {OUT_PDF} (headless browser)")


if __name__ == "__main__":
    main()
