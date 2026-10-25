# RUST-11 / msifilehash
built msifilehash-correct.msi sha256 37341c39a49c03b8ce6b03f68ca6a07b6148b29b743974275544c3eb8ae47267
msiexec /i results\RUST-11\msifilehash\msifilehash-correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7ec.msi
  log: Product: MsiSpec Test rust11 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /fo results\RUST-11\msifilehash\msifilehash-correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
msiexec /fc results\RUST-11\msifilehash\msifilehash-correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
msiexec /fe results\RUST-11\msifilehash\msifilehash-correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
RESULT msifilehash-correct: install 0; corrupted same-size file restored by /fo=true (0), /fc=false (0), /fe=true (0)
msiexec /x results\RUST-11\msifilehash\msifilehash-correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test rust11 -- Removal completed successfully.
  log: MainEngineThread is returning 0
built msifilehash-wrong.msi sha256 129c8f99cf91b94080c6773b45b18ed9db801ed6e6c667891a420efadf469059
msiexec /i results\RUST-11\msifilehash\msifilehash-wrong.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7fb.msi
  log: Product: MsiSpec Test rust11 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /fo results\RUST-11\msifilehash\msifilehash-wrong.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
msiexec /fc results\RUST-11\msifilehash\msifilehash-wrong.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
msiexec /fe results\RUST-11\msifilehash\msifilehash-wrong.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
RESULT msifilehash-wrong: install 0; corrupted same-size file restored by /fo=true (0), /fc=false (0), /fe=true (0)
msiexec /x results\RUST-11\msifilehash\msifilehash-wrong.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test rust11 -- Removal completed successfully.
  log: MainEngineThread is returning 0
built filehash-table.msi sha256 4ece941b05a9057cc51f56272f74a845ddead4dfb9266cf5fc187c8a65637245
msiexec /i results\RUST-11\msifilehash\filehash-table.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a80a.msi
  log: Product: MsiSpec Test rust11 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /fo results\RUST-11\msifilehash\filehash-table.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
msiexec /fc results\RUST-11\msifilehash\filehash-table.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
msiexec /fe results\RUST-11\msifilehash\filehash-table.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test rust11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
RESULT filehash-table: install 0; corrupted same-size file restored by /fo=true (0), /fc=false (0), /fe=true (0)
msiexec /x results\RUST-11\msifilehash\filehash-table.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test rust11 -- Removal completed successfully.
  log: MainEngineThread is returning 0
