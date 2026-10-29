# FILE-09 / ci

GitHub windows-latest runner, 2026-10-29, CI run 40881913820 (TestProbeWixIssues, a temporary probe replaced by the test named below). Test: TestMsiexecInstallsDotNames. The ICE61 warning comes from the default allowSameVersion row and is not about the names.

```text
DOT Directory: [["INSTALLDIR" "zdvtjtym|MsiBuilderProbeDot"] ["INSTALLDIR_.config" "py2x94uw|.config"] ["INSTALLDIR_.config_.x" "l65syxgd|.x"] ["INSTALLDIR_.git" "468tuqbf|.git"] ["ProgramFiles64Folder" "PFiles64"] ["TARGETDIR" "SourceDir"]]
DOT File: [["_.a.b.c" "uevfe45u.c|.a.b.c"] ["_.gitignore" "oj08q4vn.git|.gitignore"] ["app.exe" "app.exe"] ["config" "config"] ["y.txt" "y.txt"]]
DOT ICE: ICE61 warning: This product should remove only older versions of itself. The Maximum version is not less than the current product. (1.0.0 1.0.0) [Upgrade.Attributes {6E2A9C41-0B7D-4E85-A1F3-2C9D8B7E5A16}//1.0.0//513]
DOT ICE err=<nil> findings=223
DOT install=0 tree=["." ".a.b.c" ".config" ".config\\.x" ".config\\.x\\y.txt" ".git" ".git\\config" ".gitignore" "app.exe"]
DOT uninstall=0 root left=false
```
