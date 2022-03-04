# Changelog
All notable changes to this project will be documented in this file.  
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).   
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

# [0.7.0] - 2022-03-04
## Changed
- Removed warning log "gup:WARN : $GOPATH/bin or $GOBIN contains the directory". This log is unnecessary for both developers and users.

# [0.6.1] - 2022-02-26
## Changed
- Removed the progress bar being updated
# [0.6.0] - 2022-02-26
## Added
- remove subcommand: Remove the binary under $GOPATH/bin or $GOBIN"

## Changed
- Changed to use update subcommand when updating binaries
- Removed the progress bar being updated
# [0.5.0] - 2022-02-22
## Added
- list subcommand: List up command name with package path and version under $GOPATH/bin or $GOBIN
# [0.4.4] - 2022-02-22
## Added
- --file option: specify binary name to be update (e.g.:--file=subaru,gup,go)
# [0.4.3] - 2022-02-22
## Added
- Added the process to check the environment variable $GOBIN
# [0.4.2] - 2022-02-22
## Changed
- Use mattn/go-colorable for Windows.
- Changed to be able to get the HOME directory path in the Windows environment.
- Changed to output the command name when the package path could not be obtained.
# [0.4.0] - 2022-02-22
## Added
- export subcommand. 
  - When updating to the latest version, gup.conf is no longer generated. Generate gup.conf with export subcommand.

# [0.3.0] - 2022-02-22
## Added
- --dry-run option.