# CA-06 / netsh
CI run 40925575633 of PR #117 (commit ceeeb6f), job "Build & Test (windows-latest)" (Windows Server 2025 x64, elevated), 2026-10-29.
A temporary test ran each command as `cmd /c "netsh advfirewall firewall <command> 2>&1"` from PowerShell and listed the probe rules by display name afterwards.
Before each delete, `msibuilder-probe-A` and `msibuilder-probe-B` were added with `dir=in program="C:\msi-builder-probe\probe.exe"`.
The netsh usage text printed after "cannot be used as the name of a rule" is left out.
```
== add all :: netsh advfirewall firewall add rule name=all dir=in action=allow program="C:\msi-builder-probe\probe.exe" :: exit 1
The string 'all' cannot be used as the name of a rule.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B]
== add "all" :: netsh advfirewall firewall add rule name="all" dir=in action=allow program="C:\msi-builder-probe\probe.exe" :: exit 1
The string 'all' cannot be used as the name of a rule.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B]
== add ALL :: netsh advfirewall firewall add rule name=ALL dir=in action=allow program="C:\msi-builder-probe\probe.exe" :: exit 1
The string 'all' cannot be used as the name of a rule.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B]
== add "All" :: netsh advfirewall firewall add rule name="All" dir=in action=allow program="C:\msi-builder-probe\probe.exe" :: exit 1
The string 'all' cannot be used as the name of a rule.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B]
== add " all" :: netsh advfirewall firewall add rule name=" all" dir=in action=allow program="C:\msi-builder-probe\probe.exe" :: exit 0
Ok.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B] [ all]
== add "all " :: netsh advfirewall firewall add rule name="all " dir=in action=allow program="C:\msi-builder-probe\probe.exe" :: exit 0
Ok.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B] [ all] [all ]
== delete name=" all" (rule ' all' exists) :: netsh advfirewall firewall delete rule name=" all" dir=in program="C:\msi-builder-probe\probe.exe" :: exit 0
Deleted 1 rule(s). | Ok.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B] [all ]
== delete name="all " (rule 'all ' exists) :: netsh advfirewall firewall delete rule name="all " dir=in program="C:\msi-builder-probe\probe.exe" :: exit 0
Deleted 1 rule(s). | Ok.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B]
== delete "all" :: netsh advfirewall firewall delete rule name="all" dir=in program="C:\msi-builder-probe\probe.exe" :: exit 0
Deleted 2 rule(s). | Ok.
   probe rules now: (none)
== delete ALL :: netsh advfirewall firewall delete rule name=ALL dir=in program="C:\msi-builder-probe\probe.exe" :: exit 0
Deleted 2 rule(s). | Ok.
   probe rules now: (none)
== delete \"all\" :: netsh advfirewall firewall delete rule name=\"all\" dir=in program="C:\msi-builder-probe\probe.exe" :: exit 0
Deleted 2 rule(s). | Ok.
   probe rules now: (none)
== delete 'all' :: netsh advfirewall firewall delete rule name='all' dir=in program="C:\msi-builder-probe\probe.exe" :: exit 1
No rules match the specified criteria.
   probe rules now: [msibuilder-probe-A] [msibuilder-probe-B]
== delete name=all no dir :: netsh advfirewall firewall delete rule name=all program="C:\msi-builder-probe\probe.exe" :: exit 0
Deleted 2 rule(s). | Ok.
   probe rules now: (none)
== UNSCOPED delete name=all dir=in :: netsh advfirewall firewall delete rule name=all dir=in :: exit 0
Deleted 208 rule(s). | Ok.
   probe rules now: (none)
Get-NetFirewallRule : No MSFT_NetFirewallRule objects found with property 'Direction' equal to 'Inbound'.  Verify the
value of the property and retry.
At line:37 char:65
+ ... e $inBefore, after " + (Get-NetFirewallRule -Direction Inbound).Count
+                             ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : ObjectNotFound: (Inbound:Direction) [Get-NetFirewallRule], CimJobException
    + FullyQualifiedErrorId : CmdletizationQuery_NotFound_Direction,Get-NetFirewallRule

inbound rules before the unscoped delete 208, after 0
outbound rules before the unscoped delete 142, after 142
rules before 348, after 142
```
