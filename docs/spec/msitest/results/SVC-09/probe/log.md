# SVC-09: a service start conditioned on a feature's action state

- Machine: SERVER2025 (Windows Server 2025), elevated, 2026-10-30
- Package: the real-world port of the Jenkins MSI (jenkinsci/packaging),
  built with msi-builder main; the full verbose logs stayed on that machine
- Not in the test harness

The `StartService` feature has Level 1 and `INSTALLLEVEL` is 1. The service
is installed by another component; its start row sits in the generated
component that `startCondition` creates. Each probe was one first install
with `/qn /l*v`, then an uninstall.

| Probe | `startCondition` | Command line | Lines from the verbose log | Service after install |
| --- | --- | --- | --- | --- |
| A | `&StartService=3 OR !StartService=3` | none | `Feature: StartService; Installed: Absent; Request: Local; Action: Local`; the generated component (Condition `(&StartService=3 OR !StartService=3)`, Attributes 260) `Installed: Absent; Request: Local; Action: Null`; StartServices ran only `ServiceControl(Action=2)` | stopped, exit 0 |
| B | `START_SERVICE="1"` | `START_SERVICE=1` | the same feature line; the generated component `Action: Local`; `ServiceControl(Name=rw-jenkins,Action=1,Wait=1)` | running, exit 0 |

At a first install `!StartService=3` is false, so in probe A the term
`&StartService=3` evaluated false although the feature's action was Local.
The WiX build of the same package, whose start row is unconditional in the
StartService feature, left the service running.

Confirmed for a first install; repairs and upgrades were not tested.
