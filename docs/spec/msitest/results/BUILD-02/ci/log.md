# BUILD-02: reproducible builds (CI)

- GitHub `ubuntu-26.04` and `windows-latest` runners, 2026-10-29
- CI run 40941177928, commit f42c75c
- Test: `TestDeterministicAcrossProcesses` in `internal/builder/builder_test.go`

The test builds the windows_exporter example in 20 fresh processes (the
test binary started again, so map iteration order and allocation differ)
and requires one SHA-256 for all of them. It passes on both runners and
locally on Linux (`7d07daa0…` for that commit). The hashes of the two hosts
are not compared with each other, and the test does not shuffle insertion
order, so BUILD-02 is Partial. `TestDeterministic` checks two builds in
one process and that changed content changes the product and package codes.

BUILD-01 (missing source) is `TestMissingInput`: the build fails with a
diagnostic naming the file and returns no package (both asserted).
The stale-cabinet half does not apply: msi-builder writes the cabinet from
the inputs of each build (inferred from the design, not tested). BUILD-01
is confirmed for msi-builder at the S level for the missing source.
