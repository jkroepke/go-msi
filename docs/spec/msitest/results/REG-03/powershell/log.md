# REG-03 / powershell
SERVER2025 (Windows Server 2025 Standard Evaluation, 10.0.26100.1742), elevated Windows PowerShell 5.1.26100.1591, 2026-10-29.
Sources under `HKLM\SYSTEM\CurrentControlSet\Services\EventLog\Application`; every `MsiBuilderProbe*` key was deleted afterwards.
The probe events of B, C and D stay in the Application log.
```
A: New-EventLog -LogName Application -Source MsiBuilderProbeA
  values written: EventMessageFile [ExpandString] = C:\Windows\Microsoft.NET\Framework64\v4.0.30319\EventLogMessages.dll
  no TypesSupported, no CustomSource, no subkeys
  EventLog\Application\Sources: absent (neither key nor value)

B: reg add ... EventMessageFile REG_EXPAND_SZ %SystemRoot%\System32\EventCreate.exe, TypesSupported REG_DWORD 7
  [EventLog]::SourceExists('MsiBuilderProbeB')   True (LogNameFromSourceName: Application)
  Write-EventLog -Source MsiBuilderProbeB -EventId 1 -Message probe   no error, no value added to the key
  Get-EventLog -Newest 1   Message: probe, Source: MsiBuilderProbeB (Get-WinEvent: probe)
  Remove-EventLog -Source MsiBuilderProbeB   no error, key gone

C: as B plus CustomSource REG_DWORD 1
  identical to B: SourceExists True, Message: probe, Remove-EventLog removes the key

D: TypesSupported REG_DWORD 7 only, no EventMessageFile
  SourceExists True, Write-EventLog no error
  Get-EventLog Message: The description for Event ID '1' in Source 'MsiBuilderProbeD' cannot be found. [...] The following information is part of the event:'probe'
  Get-WinEvent Message: (empty)
```
