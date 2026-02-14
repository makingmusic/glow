# Roadmap

## Document Format Support

- **Fetch documents from URL**: Ability to fetch a document directly from any HTTP(S) URL and render it in the terminal.
- **Docx/PDF/ODT to text conversion**: Ability to convert Docx, PDF, and other related file formats to text and render them in the terminal. This feature may be limited to the text portion of those documents.
- **HTML rendering**: Render HTML files as readable text in the terminal, converting tags to styled output.
- **CSV/TSV table rendering**: Display tabular data files as styled, aligned tables.
- **Mermaid diagram rendering**: Render Mermaid diagram files and fenced code blocks as visual output in the terminal.

## Navigation & Reading

- **Table of contents / outline view**: Extract headings to build a navigable TOC sidebar or overlay for quick jumping within a document.
- **Reading progress persistence**: Remember scroll position per file so users can resume where they left off.
- **Collapsible sections**: Collapse and expand sections by heading level to navigate large documents more easily.

## Querying & Understanding

- **In-document grep with context**: Search with surrounding context lines displayed inline, similar to `grep -C`.

## Export & Sharing

- **Export to PDF/HTML**: Render a styled document out to PDF or HTML for sharing.

## Integration

- **Clipboard rendering**: Render markdown content directly from the clipboard (`glow --clipboard`).

## Theming

- **iTerm2 color scheme support**: Support themes from [iterm2colorschemes.com](https://iterm2colorschemes.com/), allowing users to apply a wide variety of community color schemes to Glow's rendering.

## Performance & Long Documents

- **Streaming / lazy rendering for large files**: Render large documents incrementally so they open instantly regardless of size.
