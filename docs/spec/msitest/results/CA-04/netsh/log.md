# CA-04 / netsh
CI run 40787803798 of PR #77, job "Build & Test (windows-latest)" (Windows Server 2025 x64, elevated), 2026-10-28.
Each command ran as `"%SystemRoot%\System32\netsh.exe" advfirewall firewall <command>` with the command line passed unchanged.
The netsh usage text printed after "not valid" is left out.
```
add rule name="MBProbe1" dir=in action=allow protocol=tcp localport=65001 remoteip=any,10.0.0.1
  err=exit status 1
  A specified IP address or address keyword is not valid.
show rule name="MBProbe1" verbose
  err=exit status 1
  No rules match the specified criteria.
delete rule name="MBProbe1"
  err=exit status 1
  No rules match the specified criteria.
add rule name="MBProbe2" dir=in action=allow protocol=tcp localport=65001 remoteip=10.0.0.1,any
  err=exit status 1
  A specified IP address or address keyword is not valid.
show rule name="MBProbe2" verbose
  err=exit status 1
  No rules match the specified criteria.
delete rule name="MBProbe2"
  err=exit status 1
  No rules match the specified criteria.
add rule name="MBProbe3" dir=in action=allow protocol=tcp localport=65001 remoteip=*
  err=<nil>
  Ok.
show rule name="MBProbe3" verbose
  err=<nil>
  Rule Name:                            MBProbe3
  ----------------------------------------------------------------------
  Enabled:                              Yes
  Direction:                            In
  Profiles:                             Domain,Private,Public
  Grouping:
  LocalIP:                              Any
  RemoteIP:                             Any
  Protocol:                             TCP
  LocalPort:                            65001
  RemotePort:                           Any
  Edge traversal:                       No
  InterfaceTypes:                       Any
  Security:                             NotRequired
  Rule source:                          Local Setting
  Action:                               Allow
  Ok.
delete rule name="MBProbe3"
  err=<nil>
  Deleted 1 rule(s).
  Ok.
add rule name="MBProbe4" dir=in action=allow protocol=tcp localport=65001 remoteip=*,10.0.0.1
  err=exit status 1
  A specified IP address or address keyword is not valid.
show rule name="MBProbe4" verbose
  err=exit status 1
  No rules match the specified criteria.
delete rule name="MBProbe4"
  err=exit status 1
  No rules match the specified criteria.
add rule name="MBProbe5" dir=in action=allow protocol=tcp localport=65001 remoteip=localsubnet,10.0.0.1
  err=<nil>
  Ok.
show rule name="MBProbe5" verbose
  err=<nil>
  Rule Name:                            MBProbe5
  ----------------------------------------------------------------------
  Enabled:                              Yes
  Direction:                            In
  Profiles:                             Domain,Private,Public
  Grouping:
  LocalIP:                              Any
  RemoteIP:                             LocalSubnet,10.0.0.1/32
  Protocol:                             TCP
  LocalPort:                            65001
  RemotePort:                           Any
  Edge traversal:                       No
  InterfaceTypes:                       Any
  Security:                             NotRequired
  Rule source:                          Local Setting
  Action:                               Allow
  Ok.
delete rule name="MBProbe5"
  err=<nil>
  Deleted 1 rule(s).
  Ok.
add rule name="MBProbe6" dir=in action=allow protocol=tcp localport=65001 remoteip=ANY
  err=<nil>
  Ok.
show rule name="MBProbe6" verbose
  err=<nil>
  Rule Name:                            MBProbe6
  ----------------------------------------------------------------------
  Enabled:                              Yes
  Direction:                            In
  Profiles:                             Domain,Private,Public
  Grouping:
  LocalIP:                              Any
  RemoteIP:                             Any
  Protocol:                             TCP
  LocalPort:                            65001
  RemotePort:                           Any
  Edge traversal:                       No
  InterfaceTypes:                       Any
  Security:                             NotRequired
  Rule source:                          Local Setting
  Action:                               Allow
  Ok.
delete rule name="MBProbe6"
  err=<nil>
  Deleted 1 rule(s).
  Ok.
add rule name="MBProbe7" dir=in action=allow protocol=tcp localport=65001 description="say "hi" now"
  err=<nil>
  Ok.
show rule name="MBProbe7" verbose
  err=<nil>
  Rule Name:                            MBProbe7
  ----------------------------------------------------------------------
  Description:                          say hi now
  Enabled:                              Yes
  Direction:                            In
  Profiles:                             Domain,Private,Public
  Grouping:
  LocalIP:                              Any
  RemoteIP:                             Any
  Protocol:                             TCP
  LocalPort:                            65001
  RemotePort:                           Any
  Edge traversal:                       No
  InterfaceTypes:                       Any
  Security:                             NotRequired
  Rule source:                          Local Setting
  Action:                               Allow
  Ok.
delete rule name="MBProbe7"
  err=<nil>
  Deleted 1 rule(s).
  Ok.
add rule name="MBProbe8" dir=in action=allow protocol=tcp localport=65001 description="plain text"
  err=<nil>
  Ok.
show rule name="MBProbe8" verbose
  err=<nil>
  Rule Name:                            MBProbe8
  ----------------------------------------------------------------------
  Description:                          plain text
  Enabled:                              Yes
  Direction:                            In
  Profiles:                             Domain,Private,Public
  Grouping:
  LocalIP:                              Any
  RemoteIP:                             Any
  Protocol:                             TCP
  LocalPort:                            65001
  RemotePort:                           Any
  Edge traversal:                       No
  InterfaceTypes:                       Any
  Security:                             NotRequired
  Rule source:                          Local Setting
  Action:                               Allow
  Ok.
delete rule name="MBProbe8"
  err=<nil>
  Deleted 1 rule(s).
  Ok.
add rule name="MBProbe9" dir=in action=allow protocol=tcp localport=65001 remoteip=LocalSubnet,dns
  err=<nil>
  Ok.
show rule name="MBProbe9" verbose
  err=<nil>
  Rule Name:                            MBProbe9
  ----------------------------------------------------------------------
  Enabled:                              Yes
  Direction:                            In
  Profiles:                             Domain,Private,Public
  Grouping:
  LocalIP:                              Any
  RemoteIP:                             LocalSubnet,DNS
  Protocol:                             TCP
  LocalPort:                            65001
  RemotePort:                           Any
  Edge traversal:                       No
  InterfaceTypes:                       Any
  Security:                             NotRequired
  Rule source:                          Local Setting
  Action:                               Allow
  Ok.
delete rule name="MBProbe9"
  err=<nil>
  Deleted 1 rule(s).
  Ok.
```
