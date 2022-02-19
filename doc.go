// gup - Update binaries installed by "go install"
//
// gup command update binaries installed by "go install" to the latest version.
// The gup command saves the command's package path (that is, \<PATH\> in
// "$ go install \<PATH\>") in the configuration file.
//
// The information of this package path is added to the configuration file under
// the following conditions.
// - If there is installation information in the shell history
// - If you installed the binaries through gup command
// - If the user manually edits the configuration file
//
// gup command can not obtaine all package path information from the shell history.
// So, you need to add the PATH information manually.
// `$HOME/.config/gup/gup.conf` saves the settings in the`$BINARY_NAME = $PATH` format.
// $PATH may be empty as shown below.
// ```
// cheat = github.com/cheat/cheat/cmd/cheat
// dlv =
// dlv-dap =
// ff =
// fyne_demo =
// gal = github.com/nao1215/gal/cmd/gal
// ```
// The simplest way is to write the path to the config file using your favorite editor.
// Another method is that you use the gup command to install the binaries.
// ```
// $ gup install command_package_path
// ```
// `$ gup install` is a wrapper for `$ go install`.
// The difference is that there are no options and gup always get the latest version.
// The biggest difference is that the path information is recorded in the configuration
// file of the gup command.
package main
