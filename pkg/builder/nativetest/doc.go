// Package nativetest checks built packages against Windows Installer itself:
// msi.dll opens and queries the database, and msiexec installs and removes
// the minimal example. The tests only compile on Windows; elsewhere the
// package is empty. They run only without -short.
package nativetest
