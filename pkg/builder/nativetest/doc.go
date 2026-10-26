// Package nativetest checks built packages against Windows Installer itself:
// msi.dll opens and queries the database, the Darice ICE suite runs through
// internal/ice, and msiexec installs and removes every example. The
// full-UI tests drive the wizard's Restart Manager and error dialogs through
// UIAutomation; they also need elevation and an interactive desktop session,
// and skip without them. The ICE tests need darice.cub and skip without it
// unless GO_MSI_ICE_REQUIRED is set. The tests only compile on Windows;
// elsewhere the package is empty. They run only without -short.
package nativetest
