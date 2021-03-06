# Changelog
All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/). This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

# [0.10.0] - 2022-04-17
## Added
- Added automatic generation of completion files for bash, zsh, and fish
# [0.9.3] - 2022-04-16
## Changed
- Parallelized update subcommand process
- Parallelized check subcommand process
- Simplified messages during the update/check process
- Display the latest version after an update in an easily recognizable color
- Improved error messages.
# [0.9.1] - 2022-03-19
## Changed 
- Changed the message at the time of update was incorrect, so the message was corrected.
- Changed version check result message to user-friendly one.
# [0.9.0] - 2022-03-18
## Added
- Added desktop notification: gup command will notify you on your desktop whether the update was successful or unsuccessful after the update was finished.
# [0.8.0] - 2022-03-18
## Added
- Added check subcommand: get the latest version of the binary installed by 'go install'"
# [0.7.4] - 2022-03-13
## Changed
- Fix: Bug that causes runtime error in "$ gup import"
This bug was caused by an insufficient setting of package version information.

# [0.7.2] - 2022-03-06
## Changed
- Fix: "commans" is a misspelling of "commands" (misspell) at cmd/update.go
# [0.7.2] - 2022-03-05
## Changed
- When the update is completed, the version information before and after the update will be output.
- Changed to output version information in dry run mode.
- Changed to log the detailed reason when the update failed.
# [0.7.0] - 2022-03-04
## Changed
- Removed warning log "gup:WARN : $GOPATH/bin or $GOBIN contains the directory". This log is unnecessary for both developers and users.
- Changed dry run short option from -d to -n
- Changed to be able to update the specified command without the --file option

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