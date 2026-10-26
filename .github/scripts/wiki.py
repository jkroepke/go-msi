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

# Not published: README.md files, which describe a directory to
# contributors, and the Windows test harness.
EXCLUDE = ("spec/msitest/",)

PAGES = {
    "design/yaml-schema.md": "YAML Schema",
    "design/architecture.md": "Architecture",
    "go-api.md": "Go API",
    "signing.md": "Signing",
    "spec/MSISPEC.md": "MSI Specification",
    "spec/MSISPEC-RESULTS.md": "Windows Test Results",
    "cli/go-msi.md": "Command Line",
}

# Code is copied as it is. Links are inline links, with an optional title,
# and reference definitions.
TOKEN = re.compile(
    r"(?P<code>^(?P<fence>```|~~~).*?^(?P=fence)[^\n]*$|`[^`\n]+`)"
    r"|(?P<open>\]\()(?P<target>[^)\s]+)(?P<close>(?:\s+\"[^\"]*\")?\))"
    r"|(?P<ref>^\[[^\]\n]+\]:[ \t]+)(?P<reftarget>\S+)",
    re.MULTILINE | re.DOTALL,
)


def page_name(rel):
    if rel in PAGES:
        return PAGES[rel]
    if rel.startswith("cli/go-msi_"):
        return "Command " + rel[len("cli/go-msi_") : -len(".md")].replace("_", " ")
    return os.path.basename(rel)[: -len(".md")]


def collect(docs):
    pages, assets = {}, set()
    for root, dirs, files in os.walk(docs):
        dirs.sort()
        for name in sorted(files):
            rel = os.path.relpath(os.path.join(root, name), docs).replace(os.sep, "/")
            if name == "README.md" or rel.startswith(EXCLUDE):
                continue
            if name.endswith(".md"):
                pages[rel] = page_name(rel)
            else:
                assets.add(rel)
    names = sorted(pages.values())
    duplicates = sorted({n for n in names if names.count(n) > 1})
    if duplicates:
        sys.exit("duplicate wiki page names: " + ", ".join(duplicates))
    return pages, assets


def rewrite(text, rel, docs, pages, assets, repo):
    base = os.path.dirname(rel)

    def target(link):
        if re.match(r"^[a-z][a-z0-9+.-]*:", link) or link.startswith("#"):
            return link
        path, sep, anchor = link.partition("#")
        resolved = os.path.normpath(os.path.join(base, unquote(path))).replace(os.sep, "/")
        if resolved in pages:
            return quote(pages[resolved]) + sep + anchor
        if resolved in assets:
            # Copied next to the pages under the same path.
            return quote(resolved) + sep + anchor
        source = os.path.normpath(os.path.join(docs, resolved))
        if not os.path.exists(source):
            sys.exit(f"{rel}: link to missing file {link}")
        kind = "tree" if os.path.isdir(source) else "blob"
        path = quote(os.path.relpath(source, ".").replace(os.sep, "/"))
        return f"https://github.com/{repo}/{kind}/main/{path}{sep}{anchor}"

    def token(match):
        if match.group("code"):
            return match.group(0)
        if match.group("ref"):
            return match.group("ref") + target(match.group("reftarget"))
        return match.group("open") + target(match.group("target")) + match.group("close")

    return TOKEN.sub(token, text)


def main():
    docs, wiki = sys.argv[1:3]
    repo = os.environ.get("GITHUB_REPOSITORY", "jkroepke/go-msi")
    pages, assets = collect(docs)

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
            f.write(rewrite(text, rel, docs, pages, assets, repo))

    for rel in sorted(assets):
        os.makedirs(os.path.dirname(os.path.join(wiki, rel)), exist_ok=True)
        shutil.copyfile(os.path.join(docs, rel), os.path.join(wiki, rel))


if __name__ == "__main__":
    main()
