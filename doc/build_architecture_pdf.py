from html import escape
from pathlib import Path

from docx import Document
from docx.table import Table
from docx.text.paragraph import Paragraph
from docx.oxml.ns import qn
from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER, TA_LEFT, TA_RIGHT
from reportlab.lib.pagesizes import LETTER
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch
from reportlab.platypus import (
    Image,
    KeepTogether,
    PageBreak,
    Paragraph as PdfParagraph,
    SimpleDocTemplate,
    Spacer,
    Table as PdfTable,
    TableStyle,
)


ROOT = Path(__file__).resolve().parent
DOCX = ROOT / "aspgen-architecture-blueprint.docx"
PDF = ROOT / "aspgen-architecture-blueprint.pdf"
ASSETS = ROOT / "architecture-assets"


def iter_block_items(parent):
    parent_elm = parent.element.body
    for child in parent_elm.iterchildren():
        if child.tag == qn("w:p"):
            yield Paragraph(child, parent)
        elif child.tag == qn("w:tbl"):
            yield Table(child, parent)


def page_furniture(canvas, doc):
    canvas.saveState()
    canvas.setFont("Helvetica", 8)
    canvas.setFillColor(colors.HexColor("#5B6573"))
    canvas.drawRightString(7.5 * inch, 0.55 * inch, f"Architecture Theta  •  Page {doc.page}")
    canvas.drawString(1 * inch, 10.45 * inch, "aspgen  |  Architecture Blueprint")
    canvas.restoreState()


def make_styles():
    base = getSampleStyleSheet()
    return {
        "body": ParagraphStyle("Body", parent=base["BodyText"], fontName="Helvetica", fontSize=10.5, leading=14, spaceAfter=6, textColor=colors.HexColor("#182230")),
        "h1": ParagraphStyle("H1", parent=base["Heading1"], fontName="Helvetica-Bold", fontSize=16, leading=19, spaceBefore=16, spaceAfter=8, textColor=colors.HexColor("#2E74B5")),
        "h2": ParagraphStyle("H2", parent=base["Heading2"], fontName="Helvetica-Bold", fontSize=13, leading=16, spaceBefore=12, spaceAfter=6, textColor=colors.HexColor("#2E74B5")),
        "h3": ParagraphStyle("H3", parent=base["Heading3"], fontName="Helvetica-Bold", fontSize=12, leading=14, spaceBefore=8, spaceAfter=4, textColor=colors.HexColor("#1F4D78")),
        "code": ParagraphStyle("Code", parent=base["Code"], fontName="Courier", fontSize=8.5, leading=11, leftIndent=8, rightIndent=8, spaceBefore=4, spaceAfter=8, backColor=colors.HexColor("#F2F4F7")),
        "caption": ParagraphStyle("Caption", parent=base["BodyText"], fontName="Helvetica-Oblique", fontSize=8.5, leading=11, alignment=TA_CENTER, textColor=colors.HexColor("#5B6573"), spaceAfter=8),
        "bullet": ParagraphStyle("Bullet", parent=base["BodyText"], fontName="Helvetica", fontSize=10.5, leading=14, leftIndent=18, firstLineIndent=-10, spaceAfter=4, textColor=colors.HexColor("#182230")),
        "number": ParagraphStyle("Number", parent=base["BodyText"], fontName="Helvetica", fontSize=10.5, leading=14, leftIndent=20, firstLineIndent=-14, spaceAfter=5, textColor=colors.HexColor("#182230")),
    }


def para_text(p):
    return "".join(run.text for run in p.runs)


def table_flowable(table):
    rows = []
    for row in table.rows:
        rows.append([PdfParagraph(escape(" ".join(cell.text.split())), ParagraphStyle("Cell", fontName="Helvetica", fontSize=8.4, leading=10.5, textColor=colors.HexColor("#182230"))) for cell in row.cells])
    widths = [6.5 * inch / len(rows[0]) for _ in rows[0]]
    t = PdfTable(rows, colWidths=widths, repeatRows=1, hAlign="LEFT")
    commands = [
        ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#0B2545")),
        ("TEXTCOLOR", (0, 0), (-1, 0), colors.white),
        ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
        ("FONTSIZE", (0, 0), (-1, -1), 8.4),
        ("GRID", (0, 0), (-1, -1), 0.35, colors.HexColor("#C8D0D9")),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("LEFTPADDING", (0, 0), (-1, -1), 7),
        ("RIGHTPADDING", (0, 0), (-1, -1), 7),
        ("TOPPADDING", (0, 0), (-1, -1), 6),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
    ]
    for i in range(1, len(rows)):
        commands.append(("BACKGROUND", (0, i), (-1, i), colors.HexColor("#F4F6F9" if i % 2 else "#FFFFFF")))
    t.setStyle(TableStyle(commands))
    return [Spacer(1, 5), t, Spacer(1, 8)]


def build():
    document = Document(DOCX)
    styles = make_styles()
    story = []
    image_index = 0
    for block in iter_block_items(document):
        if isinstance(block, Table):
            story.extend(table_flowable(block))
            continue
        text = para_text(block)
        has_drawing = bool(block._p.xpath(".//w:drawing"))
        if has_drawing:
            image_index += 1
            asset = ASSETS / ("gateway.png" if image_index == 1 else "flow.png")
            story.append(Image(str(asset), width=6.5 * inch, height=2.95 * inch if image_index == 1 else 2.24 * inch))
            story.append(Spacer(1, 4))
            continue
        if not text.strip():
            story.append(Spacer(1, 4))
            continue
        style_name = block.style.name if block.style else "Normal"
        if style_name == "Heading 1":
            story.append(PdfParagraph(escape(text), styles["h1"]))
        elif style_name == "Heading 2":
            story.append(PdfParagraph(escape(text), styles["h2"]))
        elif style_name == "Heading 3":
            story.append(PdfParagraph(escape(text), styles["h3"]))
        elif style_name == "List Bullet":
            story.append(PdfParagraph("• " + escape(text), styles["bullet"]))
        elif style_name == "List Number":
            story.append(PdfParagraph(escape(text), styles["number"]))
        elif text.startswith("Figure "):
            story.append(PdfParagraph(escape(text), styles["caption"]))
        elif text.startswith("aspgen ") or text.startswith("ARCHITECTURE"):
            story.append(PdfParagraph(escape(text), styles["h1"]))
        else:
            story.append(PdfParagraph(escape(text), styles["body"]))
    doc = SimpleDocTemplate(str(PDF), pagesize=LETTER, rightMargin=inch, leftMargin=inch, topMargin=0.82 * inch, bottomMargin=0.8 * inch, title="aspgen Architecture Blueprint", author="aspgen Architecture Theta")
    doc.build(story, onFirstPage=page_furniture, onLaterPages=page_furniture)
    print(PDF)


if __name__ == "__main__":
    build()
