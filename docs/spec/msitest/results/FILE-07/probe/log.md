# FILE-07 / probe
CI run 40833269994 of PR #78, job "Build & Test (windows-latest)" (Windows Server 2025 x64, elevated), 2026-10-29.
Each case was built by msi-builder before the FILE-07 check existed, run through the ICE suite (errors and warnings shown), installed with `msiexec /i … /qn`, listed, removed, then installed again with `SHORTFILENAMES=1`, listed and removed.
Every case is an x64 package with `PROBEDIR` (`snprobe<case>` under `ProgramFiles64Folder`) and its two children `DIRONE` (`DATA~1|Data One`) and `DIRTWO` (`DATA~1|Data Two`):

- a: `PROG~1.TXT|Program One.txt` and `PROG~1.TXT|Program Two.txt` in one component in `PROBEDIR`.
- b: the same two files, one component each.
- c: `one.txt` in `DIRONE` and `two.txt` in `DIRTWO`.
- d: a file `DATA~1|Data Three` and `two.txt` in `PROBEDIR`, and `three.txt` in `DIRONE`.

Log lines about the missing `Error` table (notes 2205 and 2228), source directories, property changes and the volume summary of `dir /x` are left out.
```
=== case a
ICE err=<nil>
  ICE30 error: Installation of component 'Both' would cause the target file 'PROG~1.TXT|Program One.txt' to be installed twice in '[ProgramFiles64Folder]\snprobea\' on an SFN system. This will break the component reference counting system. [File.File Two]
  ICE30 error: Installation of component 'Both' would cause the target file 'PROG~1.TXT|Program One.txt' to be installed twice in '[ProgramFiles64Folder]\snprobea\' on an SFN system. This will break the component reference counting system. [File.File One]
  ICE61 warning: This product should remove only older versions of itself. The Maximum version is not less than the current product. (1.0.0 1.0.0) [Upgrade.Attributes {3F2504E0-4F89-41D3-9A0C-0305E82C3A01}//1.0.0//513]
install []: err=<nil>
  C:\Program Files\snprobea ""
  C:\Program Files\snprobea\Program One.txt "one"
  C:\Program Files\snprobea\Program Two.txt "two"
  dir /x:

 Directory of C:\Program Files\snprobea

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM                 3 PROGRA~1.TXT Program One.txt
10/29/2026  07:07 AM                 3 PROGRA~2.TXT Program Two.txt
               2 File(s)              6 bytes

               2 File(s)              6 bytes

  log: MSI (s) (B8:EC) [07:07:32:568]: Dir (target): Key: PROBEDIR	, Object: C:\Program Files\snprobea\
  log: MSI (s) (B8:EC) [07:07:32:568]: Dir (target): Key: DIRTWO	, Object: C:\Program Files\snprobea\Data Two\
  log: MSI (s) (B8:EC) [07:07:32:568]: Dir (target): Key: DIRONE	, Object: C:\Program Files\snprobea\Data One\
  log: MSI (s) (B8:EC) [07:07:32:710]: File: C:\Program Files\snprobea\Program One.txt;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:EC) [07:07:32:716]: File: C:\Program Files\snprobea\Program Two.txt;	To be installed;	Won't patch;	No existing file
  log: Property(S): DIRONE = C:\Program Files\snprobea\Data One\
  log: Property(S): DIRTWO = C:\Program Files\snprobea\Data Two\
  log: Property(S): PROBEDIR = C:\Program Files\snprobea\
uninstall: err=<nil>
  root after uninstall: GetFileAttributesEx C:\Program Files\snprobea: The system cannot find the file specified.
install [SHORTFILENAMES=1]: err=<nil>
  C:\Program Files\snprobea ""
  C:\Program Files\snprobea\PROG~1.TXT "two"
  dir /x:

 Directory of C:\Program Files\snprobea

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM                 3              PROG~1.TXT
               1 File(s)              3 bytes

               1 File(s)              3 bytes

  log: MSI (s) (B8:C4) [07:07:33:323]: Command Line: SHORTFILENAMES=1 CURRENTDIRECTORY=D:\a\msi-builder\msi-builder\pkg\builder\nativetest CLIENTUILEVEL=3 CLIENTPROCESSID=7020
  log: MSI (s) (B8:C4) [07:07:33:410]: Dir (target): Key: PROBEDIR	, Object: C:\Program Files\snprobea\
  log: MSI (s) (B8:C4) [07:07:33:410]: Dir (target): Key: DIRTWO	, Object: C:\Program Files\snprobea\DATA~1\
  log: MSI (s) (B8:C4) [07:07:33:410]: Dir (target): Key: DIRONE	, Object: C:\Program Files\snprobea\DATA~1\
  log: MSI (s) (B8:C4) [07:07:33:515]: File: C:\Program Files\snprobea\PROG~1.TXT;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:C4) [07:07:33:520]: File: C:\Program Files\snprobea\PROG~1.TXT;	Overwrite;	Won't patch;	Existing file is unversioned and unmodified - hash doesn't match source file
  log: Property(S): DIRONE = C:\Program Files\snprobea\DATA~1\
  log: Property(S): DIRTWO = C:\Program Files\snprobea\DATA~1\
  log: Property(S): PROBEDIR = C:\Program Files\snprobea\
  log: Property(S): SHORTFILENAMES = 1
uninstall: err=<nil>
  root after uninstall: <nil>
=== case b
ICE err=<nil>
  ICE30 error: The target file 'PROG~1.TXT|Program One.txt' is installed in '[ProgramFiles64Folder]\snprobeb\' by two different components on an SFN system: 'CompOne' and 'CompTwo'. This breaks component reference counting. [File.File Two]
  ICE30 error: The target file 'PROG~1.TXT|Program One.txt' is installed in '[ProgramFiles64Folder]\snprobeb\' by two different components on an SFN system: 'CompOne' and 'CompTwo'. This breaks component reference counting. [File.File One]
  ICE61 warning: This product should remove only older versions of itself. The Maximum version is not less than the current product. (1.0.0 1.0.0) [Upgrade.Attributes {3F2504E0-4F89-41D3-9A0C-0305E82C3A02}//1.0.0//513]
install []: err=<nil>
  C:\Program Files\snprobeb ""
  C:\Program Files\snprobeb\Program One.txt "one"
  C:\Program Files\snprobeb\Program Two.txt "two"
  dir /x:

 Directory of C:\Program Files\snprobeb

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM                 3 PROGRA~1.TXT Program One.txt
10/29/2026  07:07 AM                 3 PROGRA~2.TXT Program Two.txt
               2 File(s)              6 bytes

               2 File(s)              6 bytes

  log: MSI (s) (B8:4C) [07:07:37:723]: Dir (target): Key: PROBEDIR	, Object: C:\Program Files\snprobeb\
  log: MSI (s) (B8:4C) [07:07:37:723]: Dir (target): Key: DIRTWO	, Object: C:\Program Files\snprobeb\Data Two\
  log: MSI (s) (B8:4C) [07:07:37:723]: Dir (target): Key: DIRONE	, Object: C:\Program Files\snprobeb\Data One\
  log: MSI (s) (B8:4C) [07:07:37:805]: File: C:\Program Files\snprobeb\Program One.txt;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:4C) [07:07:37:811]: File: C:\Program Files\snprobeb\Program Two.txt;	To be installed;	Won't patch;	No existing file
  log: Property(S): DIRONE = C:\Program Files\snprobeb\Data One\
  log: Property(S): DIRTWO = C:\Program Files\snprobeb\Data Two\
  log: Property(S): PROBEDIR = C:\Program Files\snprobeb\
uninstall: err=<nil>
  root after uninstall: GetFileAttributesEx C:\Program Files\snprobeb: The system cannot find the file specified.
install [SHORTFILENAMES=1]: err=<nil>
  C:\Program Files\snprobeb ""
  C:\Program Files\snprobeb\PROG~1.TXT "two"
  dir /x:

 Directory of C:\Program Files\snprobeb

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM                 3              PROG~1.TXT
               1 File(s)              3 bytes

               1 File(s)              3 bytes

  log: MSI (s) (B8:54) [07:07:38:442]: Command Line: SHORTFILENAMES=1 CURRENTDIRECTORY=D:\a\msi-builder\msi-builder\pkg\builder\nativetest CLIENTUILEVEL=3 CLIENTPROCESSID=2520
  log: MSI (s) (B8:54) [07:07:38:502]: Dir (target): Key: PROBEDIR	, Object: C:\Program Files\snprobeb\
  log: MSI (s) (B8:54) [07:07:38:502]: Dir (target): Key: DIRTWO	, Object: C:\Program Files\snprobeb\DATA~1\
  log: MSI (s) (B8:54) [07:07:38:502]: Dir (target): Key: DIRONE	, Object: C:\Program Files\snprobeb\DATA~1\
  log: MSI (s) (B8:54) [07:07:38:580]: File: C:\Program Files\snprobeb\PROG~1.TXT;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:54) [07:07:38:586]: File: C:\Program Files\snprobeb\PROG~1.TXT;	Overwrite;	Won't patch;	Existing file is unversioned and unmodified - hash doesn't match source file
  log: Property(S): DIRONE = C:\Program Files\snprobeb\DATA~1\
  log: Property(S): DIRTWO = C:\Program Files\snprobeb\DATA~1\
  log: Property(S): PROBEDIR = C:\Program Files\snprobeb\
  log: Property(S): SHORTFILENAMES = 1
uninstall: err=<nil>
  root after uninstall: <nil>
=== case c
ICE err=<nil>
  ICE61 warning: This product should remove only older versions of itself. The Maximum version is not less than the current product. (1.0.0 1.0.0) [Upgrade.Attributes {3F2504E0-4F89-41D3-9A0C-0305E82C3A03}//1.0.0//513]
install []: err=<nil>
  C:\Program Files\snprobec ""
  C:\Program Files\snprobec\Data One ""
  C:\Program Files\snprobec\Data One\one.txt "one"
  C:\Program Files\snprobec\Data Two ""
  C:\Program Files\snprobec\Data Two\two.txt "two"
  dir /x:

 Directory of C:\Program Files\snprobec

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM    <DIR>          DATAON~1     Data One
10/29/2026  07:07 AM    <DIR>          DATATW~1     Data Two
               0 File(s)              0 bytes

 Directory of C:\Program Files\snprobec\Data One

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM                 3              one.txt
               1 File(s)              3 bytes

 Directory of C:\Program Files\snprobec\Data Two

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM                 3              two.txt
               1 File(s)              3 bytes

               2 File(s)              6 bytes

  log: MSI (s) (B8:84) [07:07:43:254]: Dir (target): Key: PROBEDIR	, Object: C:\Program Files\snprobec\
  log: MSI (s) (B8:84) [07:07:43:254]: Dir (target): Key: DIRTWO	, Object: C:\Program Files\snprobec\Data Two\
  log: MSI (s) (B8:84) [07:07:43:254]: Dir (target): Key: DIRONE	, Object: C:\Program Files\snprobec\Data One\
  log: MSI (s) (B8:84) [07:07:43:354]: File: C:\Program Files\snprobec\Data One\one.txt;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:84) [07:07:43:362]: File: C:\Program Files\snprobec\Data Two\two.txt;	To be installed;	Won't patch;	No existing file
  log: Property(S): DIRONE = C:\Program Files\snprobec\Data One\
  log: Property(S): DIRTWO = C:\Program Files\snprobec\Data Two\
  log: Property(S): PROBEDIR = C:\Program Files\snprobec\
uninstall: err=<nil>
  root after uninstall: GetFileAttributesEx C:\Program Files\snprobec: The system cannot find the file specified.
install [SHORTFILENAMES=1]: err=<nil>
  C:\Program Files\snprobec ""
  C:\Program Files\snprobec\DATA~1 ""
  C:\Program Files\snprobec\DATA~1\one.txt "one"
  C:\Program Files\snprobec\DATA~1\two.txt "two"
  dir /x:

 Directory of C:\Program Files\snprobec

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM    <DIR>                       DATA~1
               0 File(s)              0 bytes

 Directory of C:\Program Files\snprobec\DATA~1

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM                 3              one.txt
10/29/2026  07:07 AM                 3              two.txt
               2 File(s)              6 bytes

               2 File(s)              6 bytes

  log: MSI (s) (B8:6C) [07:07:43:849]: Command Line: SHORTFILENAMES=1 CURRENTDIRECTORY=D:\a\msi-builder\msi-builder\pkg\builder\nativetest CLIENTUILEVEL=3 CLIENTPROCESSID=8768
  log: MSI (s) (B8:6C) [07:07:43:910]: Dir (target): Key: PROBEDIR	, Object: C:\Program Files\snprobec\
  log: MSI (s) (B8:6C) [07:07:43:910]: Dir (target): Key: DIRTWO	, Object: C:\Program Files\snprobec\DATA~1\
  log: MSI (s) (B8:6C) [07:07:43:910]: Dir (target): Key: DIRONE	, Object: C:\Program Files\snprobec\DATA~1\
  log: MSI (s) (B8:6C) [07:07:43:979]: File: C:\Program Files\snprobec\DATA~1\one.txt;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:6C) [07:07:43:988]: File: C:\Program Files\snprobec\DATA~1\two.txt;	To be installed;	Won't patch;	No existing file
  log: Property(S): DIRONE = C:\Program Files\snprobec\DATA~1\
  log: Property(S): DIRTWO = C:\Program Files\snprobec\DATA~1\
  log: Property(S): PROBEDIR = C:\Program Files\snprobec\
  log: Property(S): SHORTFILENAMES = 1
uninstall: err=<nil>
  root after uninstall: GetFileAttributesEx C:\Program Files\snprobec: The system cannot find the file specified.
=== case d
ICE err=<nil>
  ICE61 warning: This product should remove only older versions of itself. The Maximum version is not less than the current product. (1.0.0 1.0.0) [Upgrade.Attributes {3F2504E0-4F89-41D3-9A0C-0305E82C3A04}//1.0.0//513]
install []: err=<nil>
  C:\Program Files\snprobed ""
  C:\Program Files\snprobed\Data One ""
  C:\Program Files\snprobed\Data One\three.txt "two"
  C:\Program Files\snprobed\Data Three "one"
  C:\Program Files\snprobed\two.txt "two"
  dir /x:

 Directory of C:\Program Files\snprobed

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM    <DIR>          DATAON~1     Data One
10/29/2026  07:07 AM                 3 DATATH~1     Data Three
10/29/2026  07:07 AM                 3              two.txt
               2 File(s)              6 bytes

 Directory of C:\Program Files\snprobed\Data One

10/29/2026  07:07 AM    <DIR>                       .
10/29/2026  07:07 AM    <DIR>                       ..
10/29/2026  07:07 AM                 3              three.txt
               1 File(s)              3 bytes

               3 File(s)              9 bytes

  log: MSI (s) (B8:A8) [07:07:50:972]: Dir (target): Key: PROBEDIR	, Object: C:\Program Files\snprobed\
  log: MSI (s) (B8:A8) [07:07:50:972]: Dir (target): Key: DIRTWO	, Object: C:\Program Files\snprobed\Data Two\
  log: MSI (s) (B8:A8) [07:07:50:972]: Dir (target): Key: DIRONE	, Object: C:\Program Files\snprobed\Data One\
  log: MSI (s) (B8:A8) [07:07:51:040]: File: C:\Program Files\snprobed\Data Three;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:A8) [07:07:51:044]: File: C:\Program Files\snprobed\two.txt;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:A8) [07:07:51:046]: File: C:\Program Files\snprobed\Data One\three.txt;	To be installed;	Won't patch;	No existing file
  log: Property(S): DIRONE = C:\Program Files\snprobed\Data One\
  log: Property(S): DIRTWO = C:\Program Files\snprobed\Data Two\
  log: Property(S): PROBEDIR = C:\Program Files\snprobed\
uninstall: err=<nil>
  root after uninstall: GetFileAttributesEx C:\Program Files\snprobed: The system cannot find the file specified.
install [SHORTFILENAMES=1]: err=exit status 1603
  dir /x:
File Not Found

  log: MSI (s) (B8:60) [07:07:51:543]: Command Line: SHORTFILENAMES=1 CURRENTDIRECTORY=D:\a\msi-builder\msi-builder\pkg\builder\nativetest CLIENTUILEVEL=3 CLIENTPROCESSID=8144
  log: MSI (s) (B8:60) [07:07:51:602]: Dir (target): Key: PROBEDIR	, Object: C:\Program Files\snprobed\
  log: MSI (s) (B8:60) [07:07:51:602]: Dir (target): Key: DIRTWO	, Object: C:\Program Files\snprobed\DATA~1\
  log: MSI (s) (B8:60) [07:07:51:602]: Dir (target): Key: DIRONE	, Object: C:\Program Files\snprobed\DATA~1\
  log: MSI (s) (B8:60) [07:07:51:705]: File: C:\Program Files\snprobed\DATA~1;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:60) [07:07:51:710]: File: C:\Program Files\snprobed\two.txt;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:60) [07:07:51:713]: File: C:\Program Files\snprobed\DATA~1\three.txt;	To be installed;	Won't patch;	No existing file
  log: MSI (s) (B8:60) [07:07:51:713]: Product: SN Probe d -- Error 1312. Cannot create the directory 'C:\Program Files\snprobed\DATA~1'.  A file with this name already exists.  Please rename or remove the file and click retry, or click Cancel to exit.
  log: Error 1312. Cannot create the directory 'C:\Program Files\snprobed\DATA~1'.  A file with this name already exists.  Please rename or remove the file and click retry, or click Cancel to exit.
  log: MSI (s) (B8:60) [07:07:51:731]: Error in rollback skipped.	Return: 5
  log: Property(S): DIRONE = C:\Program Files\snprobed\DATA~1\
  log: Property(S): DIRTWO = C:\Program Files\snprobed\DATA~1\
  log: Property(S): PROBEDIR = C:\Program Files\snprobed\
  log: Property(S): SHORTFILENAMES = 1
uninstall: err=exit status 1605 This action is only valid for products that are currently installed.

  root after uninstall: GetFileAttributesEx C:\Program Files\snprobed: The system cannot find the file specified.
FAIL
coverage: 66.2% of statements in ./...
FAIL	github.com/jkroepke/msi-builder/pkg/builder/nativetest	298.328s
ok  	github.com/jkroepke/msi-builder/pkg/cab	142.020s	coverage: 13.2% of statements in ./...
ok  	github.com/jkroepke/msi-builder/pkg/cfb	32.901s	coverage: 6.1% of statements in ./...
ok  	github.com/jkroepke/msi-builder/pkg/config	1.718s	coverage: 21.6% of statements in ./...
ok  	github.com/jkroepke/msi-builder/pkg/msidb	3.855s	coverage: 13.0% of statements in ./...
ok  	github.com/jkroepke/msi-builder/pkg/sign	1.672s	coverage: 9.7% of statements in ./...
ok  	github.com/jkroepke/msi-builder/pkg/summary	1.142s	coverage: 1.5% of statements in ./...
FAIL
##[error]Process completed with exit code 1.
##[group]Run actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9
with:
  name: install-snapshots
  path: D:\a\_temp/install-snapshots/
  if-no-files-found: ignore
  compression-level: 6
  overwrite: false
  include-hidden-files: false
  archive: true
env:
  GOTOOLCHAIN: local
  MSI_BUILDER_ICE_CUB: C:\ice\darice.cub
  MSI_BUILDER_ICE_REQUIRED: 1
##[endgroup]
No files were found with the provided path: D:\a\_temp/install-snapshots/. No artifacts will be uploaded.
Post job cleanup.
[command]"C:\Program Files\Git\bin\git.exe" version
git version 2.56.0.windows.2
Temporarily overriding HOME='D:\a\_temp\96b235f5-1711-4087-ba2d-ce3aa59e1337' before making global git config changes
Adding repository directory to the temporary git global config as a safe directory
[command]"C:\Program Files\Git\bin\git.exe" config --global --add safe.directory D:\a\msi-builder\msi-builder
Removing SSH command configuration
[command]"C:\Program Files\Git\bin\git.exe" config --local --name-only --get-regexp core\.sshCommand
[command]"C:\Program Files\Git\bin\git.exe" submodule foreach --recursive "sh -c \"git config --local --name-only --get-regexp 'core\.sshCommand' && git config --local --unset-all 'core.sshCommand' || :\""
Removing HTTP extra header
[command]"C:\Program Files\Git\bin\git.exe" config --local --name-only --get-regexp http\.https\:\/\/github\.com\/\.extraheader
[command]"C:\Program Files\Git\bin\git.exe" submodule foreach --recursive "sh -c \"git config --local --name-only --get-regexp 'http\.https\:\/\/github\.com\/\.extraheader' && git config --local --unset-all 'http.https://github.com/.extraheader' || :\""
Removing includeIf entries pointing to credentials config files
[command]"C:\Program Files\Git\bin\git.exe" config --local --name-only --get-regexp ^includeIf\.gitdir:
[command]"C:\Program Files\Git\bin\git.exe" submodule foreach --recursive "git config --local --show-origin --name-only --get-regexp remote.origin.url"
Cleaning up orphan processes
```
