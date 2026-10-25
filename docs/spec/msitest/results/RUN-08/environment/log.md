# RUN-08 / environment
built pkg.msi sha256 a7901c93aa8018c5246bc5900312df12e9ec6fd940da75abb8df30f466632745
msiexec /i results\RUN-08\environment\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a636.msi
  log: Product: MsiSpec Test run08 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
  =-*MSISPEC_SET         value "one"        pre ""         -> after install "one" present=true
  =*MSISPEC_PERM         value "two"        pre ""         -> after install "two" present=true
  +-*MSISPEC_PLUS        value "new"        pre "existing" -> after install "existing" present=true
  =-*MSISPEC_PATH        value "[~];added"  pre "base"     -> after install "base;added" present=true
  =-*MSISPEC_PRE         value "added;[~]"  pre "base"     -> after install "added;base" present=true
  -*MSISPEC_MINUS        value "x"          pre ""         -> after install "" present=false
  *MSISPEC_NOPREFIX      value "seven"      pre ""         -> after install "seven" present=true
  !-*MSISPEC_BANG        value ""           pre "gone"     -> after install "" present=false
msiexec /x results\RUN-08\environment\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run08 -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall 0
  =-*MSISPEC_SET         -> after uninstall "" present=false
  =*MSISPEC_PERM         -> after uninstall "two" present=true
  +-*MSISPEC_PLUS        -> after uninstall "existing" present=true
  =-*MSISPEC_PATH        -> after uninstall "base" present=true
  =-*MSISPEC_PRE         -> after uninstall "base" present=true
  -*MSISPEC_MINUS        -> after uninstall "" present=false
  *MSISPEC_NOPREFIX      -> after uninstall "" present=false
  !-*MSISPEC_BANG        -> after uninstall "" present=false
