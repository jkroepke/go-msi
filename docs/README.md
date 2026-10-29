# Documentation of msi-builder

With the exception of `README.md` files and the Windows test harness in
`spec/msitest`, the files in this directory are published to the
[msi-builder GitHub Wiki](https://github.com/jkroepke/msi-builder/wiki) by
`.github/workflows/wiki.yaml`.

- `Home.md` is the wiki landing page.
- `_Sidebar.md` defines the navigation shown on every page.
- `_Footer.md` links readers back to the repository where documentation
  changes are reviewed. It starts with an empty line.
- `Getting Started.md` is the first-package journey. Reference pages stay
  focused and link back to it when helpful.
- `YAML Schema.md` is the configuration guide: the smallest file and one
  recipe per task, in plain words. `TestGuideExamples`
  (`cmd/msi-builder/guide_test.go`) builds every YAML block of it, the later
  ones as parts of the first, and fails on a warning. Keep table rows,
  generated IDs, ICE details and spec case IDs in `design/yaml-schema.md`.
- `cli/` is generated with `go generate ./cmd/msi-builder`; do not edit it by hand.

The wiki has one flat list of page names. `.github/scripts/wiki.py` gives
pages in subdirectories a readable name (its `PAGES` map, for example
`design/yaml-schema.md` becomes "YAML Schema Design") and drops the first line of
every page, because the wiki shows the page name as the title. Keep the title
on the first line so the files read well in the repository.

Link between documentation files with relative `.md` paths, as everywhere
else in the repository, with `%20` for spaces:

```markdown
[Getting Started](Getting%20Started.md)
[YAML Schema Design](design/yaml-schema.md#41-product)
```

The publishing script turns these into wiki page links. Images and other
files under `docs` are copied next to the pages, and links to other
repository files become links to the file on GitHub. Code blocks and inline
code are left alone. The script fails on a link to a missing file and on two
files with the same page name.

When adding or renaming a page, update `Home.md`, `_Sidebar.md` and, for a
page in a subdirectory, the `PAGES` map.
