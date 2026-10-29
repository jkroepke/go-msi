# SVC-05 / ci

GitHub windows-latest runner, 2026-10-29, CI run 40885964831 (TestProbeWixIssues, a temporary probe replaced by TestMsiexecInstallsDisabledService). One service with `start: disabled`, built before startOnInstall got its new default: once with the old default (`ServiceControl` start event 1), once with `control: { startOnInstall: false }`.

```text
SVC disabled "": install=1603 state=""
  MSI (s) (98:F8) [13:56:41:708]: Doing action: StartServices
  MSI (s) (98:F8) [13:56:41:756]: Executing op: ActionStart(Name=StartServices,Description=Starting services Service: [1],)
  MSI (s) (98:F8) [13:57:11:798]: Product: MsiBuilderProbeSvc -- Error 1920. Service 'MsiBuilderProbeSvc' (MsiBuilderProbeSvc) failed to start.  Verify that you have sufficient privileges to start system services.
  Action ended 13:57:11: InstallFinalize. Return value 3.
  Action ended 13:57:11: INSTALL. Return value 3.
SVC disabled ", control: { startOnInstall: false }": install=0 state="STATE              : 1  STOPPED"
```

The failing start took 30 seconds (13:56:41.756 to 13:57:11.798) before the 1920 and the rollback. The probe did not log the start type; `TestMsiexecInstallsDisabledService` checks it with `sc qc`.
