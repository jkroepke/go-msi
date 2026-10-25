# CFB-06 / sibling-order
built native.msi sha256 33e8772237edc36075360da4649a78be24d07e5c5f67ae6b1d93642e6d207b9b
variant swapped-top-children.msi sha256 dbed0290ca296f9f429cbd5cdfd954a1f5bc7581c2a11fe30c35de72a001f033
our reader (full walk, no tree search): opens fine
swapped: MsiOpenDatabaseW -> MsiOpenDatabaseW: 110
msiexec /i results\CFB-06\sibling-order\swapped-top-children.msi -> exit 1620
  log: MainEngineThread is returning 1620
swapped: install 1620
RESULT unordered siblings -> install 1620
writer rejects case-only duplicates: duplicate sibling name
