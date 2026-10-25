# DB-03 / property-row-order
built sorted.msi sha256 9e23be72455b43a5a5fe575bfcf87c468eb4347c2c8c88deee17c633ed4f6c2c
control Property rows sorted=true (7 rows)
shuffled.msi sha256 5d889645329e7d8141cf6aa7e00d12fc96cdbc8615d8c4e5c216cb70f88a07fe (Property rows 0 and 6 swapped, nothing else)
sorted: ProductCode query -> [[{96784EC4-CDEB-4AD3-AD94-CC74AEC16B41}]] err=<nil>
sorted: all Property rows -> 7 err=<nil>
shuffled: ProductCode query -> [] err=MsiDatabaseOpenViewW: 1615 (1: 2229 2: results\DB-03\property-row-order\shuffled.msi 3: Property 4: SELECT `Value` FROM `Property` WHERE `Property`='ProductCode' )
shuffled: all Property rows -> 0 err=MsiDatabaseOpenViewW: 1615 (1: 2229 2: results\DB-03\property-row-order\shuffled.msi 3: Property 4: SELECT `Property` FROM `Property` )
msiexec /i results\DB-03\property-row-order\shuffled.msi -> exit 1605
  log: Note: 1: 2203 2: C:\WINDOWS\Installer\inprogressinstallinfo.ipi 3: -2147287038
  log: Note: 1: 2211 2:  3: Property
  log: Note: 1: 2262 2: DigitalSignature 3: -2147287038
  log: Note: 1: 2205 2:  3: Control
  log: Product:  -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT shuffled install exit 1605 (spec expects 1605 with Note 2211)
