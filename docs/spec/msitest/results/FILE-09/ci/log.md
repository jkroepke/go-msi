# FILE-09 / ci

GitHub windows-latest runner, 2026-10-29, CI run 40881913820 (TestProbeWixIssues, a temporary probe replaced by the test named below). Test: TestMsiexecVerifiesChecksum. app.exe has checksum: true (File.Attributes 0x600); the checksum is wrong or the value imagehlp computes; MSICHECKCRCS was passed on the command line or not.

```text
CRC correct=false "" attrs=[[1536]]: install=0 exists=true
  repair /fc=0
CRC correct=false "MSICHECKCRCS=1" attrs=[[1536]]: install=1603 exists=false
CRC correct=true "MSICHECKCRCS=1" attrs=[[1536]]: install=0 exists=true
  repair /fc=0
CRC correct=true "" attrs=[[1536]]: install=0 exists=true
  repair /fc=0
```
