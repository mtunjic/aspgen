# aspgen Document Theme

Default visual system for future aspgen architecture, product, and engineering documents.

## Design direction

Professional technical brief with restrained blue architecture accents, generous whitespace, strong hierarchy, readable code blocks, and diagrams that explain relationships rather than decorate pages.

## Page and typography tokens

- Page: US Letter, portrait.
- Margins: 1 inch on all sides.
- Header/footer distance: 0.492 inch.
- Content width: 6.5 inches / 9360 DXA.
- Base font: Calibri, 11 pt.
- Body: left aligned, 6 pt after, 1.10 line spacing.
- Heading 1: 16 pt, bold, `#2E74B5`, 16 pt before, 8 pt after.
- Heading 2: 13 pt, bold, `#2E74B5`, 12 pt before, 6 pt after.
- Heading 3: 12 pt, bold, `#1F4D78`, 8 pt before, 4 pt after.

## Color palette

- Navy: `#0B2545` — title bars, primary anchors, table headers.
- Architecture blue: `#2E74B5` — headings, links, active boundaries.
- Dark blue: `#1F4D78` — subheadings and secondary emphasis.
- Gold: `#B7831E` — kickers, decision labels, selected accents.
- Ink: `#182230` — body text.
- Muted gray: `#5B6573` — captions, metadata, footer text.
- Pale blue-gray: `#E8EEF5` — callouts and selected panels.
- Light gray: `#F4F6F9` — alternating table rows and quiet backgrounds.

## First-page pattern

Use a technical `memo_masthead`:

1. Small uppercase gold kicker.
2. Large navy document title.
3. Gray subtitle describing the document job.
4. Compact metadata row: document, audience, status/version.
5. Optional pale-blue thesis or decision callout.

Avoid decorative title rules, excessive cover whitespace, or ornamental graphics.

## Content components

- Use real heading styles and real numbered/bulleted lists.
- Use tables only for comparisons, matrices, metadata, or repeated records.
- Table width: 9360 DXA; table indent: 120 DXA; cell margins: top/bottom 80-90 DXA, start/end 120 DXA.
- Table header: navy fill with white bold text.
- Table body: white and pale gray alternating rows with thin gray borders.
- Callouts: pale blue-gray fill, 4-5 px blue left accent, bold uppercase label.
- Code blocks: dark navy background, monospace font, pale text, rounded corners in HTML, restrained padding in PDF/DOCX.
- Diagrams: blue/navy nodes, gold for domain decisions, gray-blue arrows, short labels, white background.
- Captions: 8.5-9 pt muted italic centered text.

## Document behavior

- Include a decision-oriented summary near the beginning.
- Prefer one strong visual every 2-3 pages when relationships are complex.
- Keep examples concrete with commands, trees, dependency arrows, and representative code.
- End with recommendations, checklists, or next actions.
- Produce synchronized Markdown, HTML, PDF, and DOCX versions when requested.

## Reference implementation

The visual reference is:

`doc/aspgen-architecture-blueprint.docx`

The theme is based on the `standard_business_brief` preset with the `memo_masthead` header pattern and named aspgen color/code/diagram overrides.
