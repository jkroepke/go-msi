#!/usr/bin/env python3
"""Publish docs/ to the GitHub wiki.

The wiki has one flat namespace of page names, so every Markdown file under
docs/ becomes one page. PAGES gives the pages a readable name; other files
keep their file name. Links between documentation files are relative .md
paths in the repository and become page links on the wiki; links to other
repository files become links to the file on GitHub.

Usage: wiki.py DOCS WIKI
"""

import os
import re
import shutil
import sys
from urllib.parse import quote, unquote

# Not published: the contributor README of docs/ and the Windows test harness.
EXCLUDE = ("README.md", "spec/msitest/")

PAGES = {
    "design/yaml-schema.md": "YAML Schema",
    "design/architecture.md": "Architecture",
    "go-api.md": "Go API",
    "signing.md": "Signing",
    "spec/MSISPEC.md": "MSI Specification",
    "spec/MSISPEC-RESULTS.md": "Windows Test Results",
    "cli/go-msi.md": "Command Line",
}

LINK = re.compile(r"\]\(([^)\s]+)\)")


def page_name(rel):
    if rel in PAGES:
        return PAGES[rel]
    if rel.startswith("cli/go-msi_"):
        return "Command " + rel[len("cli/go-msi_") : -len(".md")].replace("_", " ")
    return os.path.basename(rel)[: -len(".md")]


def collect(docs):
    pages = {}
    for root, dirs, files in os.walk(docs):
        dirs.sort()
        for name in sorted(files):
            rel = os.path.relpath(os.path.join(root, name), docs).replace(os.sep, "/")
            if not name.endswith(".md") or rel.startswith(EXCLUDE):
                continue
            pages[rel] = page_name(rel)
    names = sorted(pages.values())
    duplicates = sorted({n for n in names if names.count(n) > 1})
    if duplicates:
        sys.exit("duplicate wiki page names: " + ", ".join(duplicates))
    return pages


def rewrite(text, rel, docs, pages, repo):
    base = os.path.dirname(rel)

    def link(match):
        target = match.group(1)
        if re.match(r"^[a-z][a-z0-9+.-]*:", target) or target.startswith("#"):
            return match.group(0)
        path, sep, anchor = target.partition("#")
        resolved = os.path.normpath(os.path.join(base, unquote(path))).replace(os.sep, "/")
        if resolved in pages:
            return "](" + quote(pages[resolved]) + sep + anchor + ")"
        source = os.path.normpath(os.path.join(docs, resolved))
        if not os.path.exists(source):
            sys.exit(f"{rel}: link to missing file {target}")
        kind = "tree" if os.path.isdir(source) else "blob"
        path = quote(os.path.relpath(source, ".").replace(os.sep, "/"))
        return f"](https://github.com/{repo}/{kind}/main/{path}{sep}{anchor})"

    return LINK.sub(link, text)


def main():
    docs, wiki = sys.argv[1:3]
    repo = os.environ.get("GITHUB_REPOSITORY", "jkroepke/go-msi")
    pages = collect(docs)

    for entry in os.listdir(wiki):
        if entry != ".git":
            path = os.path.join(wiki, entry)
            shutil.rmtree(path) if os.path.isdir(path) else os.remove(path)

    for rel, name in pages.items():
        with open(os.path.join(docs, rel), encoding="utf-8") as f:
            # The wiki shows the page name as the title, so the first line
            # (the title in the repository) is dropped.
            text = "".join(f.readlines()[1:]).lstrip("\n")
        with open(os.path.join(wiki, name + ".md"), "w", encoding="utf-8") as f:
            f.write(rewrite(text, rel, docs, pages, repo))


if __name__ == "__main__":
    main()
