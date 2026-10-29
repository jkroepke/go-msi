// Package nativetest checks built packages against Windows Installer itself:
// msi.dll opens and queries the database, the Darice ICE suite runs through
// internal/ice, and msiexec installs and removes every example while a
// snapshot of the machine state is compared with testdata/install. The
// full-UI tests drive the wizard's Restart Manager and error dialogs through
// UIAutomation; they also need elevation and an interactive desktop session,
// and skip without them. The ICE tests need darice.cub and skip without it
// unless MSI_BUILDER_ICE_REQUIRED is set. The tests only compile on Windows and
// run only without -short, except TestScopeOwns, which checks on every
// platform which entries the install snapshots record.
package nativetest
