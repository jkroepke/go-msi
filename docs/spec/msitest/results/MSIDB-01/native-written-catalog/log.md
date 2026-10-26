# MSIDB-01 / native-written-catalog
built control.msi sha256 1bc65e1fbdda24a25053a0cdf77090adfd70b6a2b96e5e043e397d010c448d54
CFB major 4, sector 4096, codepage 0, longRefs false, pool entries 92
free pool slots: 4
native pool sorted by bytes: false (first ids: ["" "" "" "" "Component" "ComponentId" "Directory_" "Attributes" "Condition" "KeyPath" "C_f_readme" "{44512A7C-49BA-3EE9-A43F-183A47263A92}"])
refcount mismatches (counting _Tables and _Columns): 0
raw _Tables order: [Component Directory Feature FeatureComponents File InstallExecuteSequence Media Property]
native _Tables order: [Component Directory Feature FeatureComponents File InstallExecuteSequence Media Property]
  Component                rows=1 sorted=true equal-to-native=true types=Component=0x2d48 ComponentId=0x1d26 Directory_=0x0d48 Attributes=0x0502 Condition=0x1dff KeyPath=0x1d48
  Directory                rows=3 sorted=true equal-to-native=true types=Directory=0x2d48 Directory_Parent=0x1d48 DefaultDir=0x0fff
  Feature                  rows=1 sorted=true equal-to-native=true types=Feature=0x2d26 Feature_Parent=0x1d26 Title=0x1f40 Description=0x1fff Display=0x1502 Level=0x0502 Directory_=0x1d48 Attributes=0x0502
  FeatureComponents        rows=1 sorted=true equal-to-native=true types=Feature_=0x2d26 Component_=0x2d48
  File                     rows=1 sorted=true equal-to-native=true types=File=0x2d48 Component_=0x0d48 FileName=0x0fff FileSize=0x0104 Version=0x1d48 Language=0x1d14 Attributes=0x1502 Sequence=0x0104
  InstallExecuteSequence   rows=26 sorted=true equal-to-native=true types=Action=0x2d48 Condition=0x1dff Sequence=0x1502
  Media                    rows=1 sorted=true equal-to-native=true types=DiskId=0x2502 LastSequence=0x0104 DiskPrompt=0x1f40 Cabinet=0x1dff VolumeLabel=0x1d20 Source=0x1d48
  Property                 rows=7 sorted=true equal-to-native=true types=Property=0x2d48 Value=0x0f00
streams: [T]Component(12) [T]Directory(18) [T]Feature(16) [T]FeatureComponents(4) [T]File(20) [T]InstallExecuteSequence(156) [T]Media(14) [T]Property(28) cab1.cab(109) \u0005SummaryInformation(368) [T]_Tables(16) [T]_Columns(304) [T]_StringPool(372) [T]_StringData(1044)
RESULT raw decode equals native query for every table: true
