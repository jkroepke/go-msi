// Command msitest runs the Windows validation cases of MSISPEC.md Appendix C
// against the native Windows Installer on the machine it runs on.
//
// It builds every fixture itself: packages through msi.dll's own importer
// (MsiDatabaseImportW), container and cabinet variants through the CFB,
// cabinet and table code in this directory, which is written from the text of
// MSISPEC.md only. It needs no WiX or msitools.
//
// Run it elevated on a disposable Windows machine; the installation cases
// (INST, FILE, UPG, SVC and others) install and uninstall throwaway products,
// a test service, registry values and machine environment variables named
// MsiSpec*/MSISPEC_*, and remove them again.
//
//	go build -o msitest.exe .
//	go build -o testsvc/testsvc.exe ./testsvc
//	.\msitest.exe              # lists the cases
//	.\msitest.exe VEC MSIDB-01    # runs cases; output goes to results/<case>/<variant>/
//
// Each variant directory receives log.md (committed) plus the fixtures,
// msiexec verbose logs and extracted files (ignored by git).
package main
