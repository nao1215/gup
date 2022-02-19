[![Build](https://github.com/nao1215/gup/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/build.yml)
[![UnitTest](https://github.com/nao1215/gup/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/unit_test.yml)
# gup - Update binaries installed by "go install"
**gup** command update binaries installed by "go install" to the latest version. 
The gup command saves the command's package path (that is, \<PATH\> in `$ go install <PATH>`) in the configuration file.  

The information of this package path is added to the configuration file under the following conditions. 
- If there is installation information in the shell history
- If you installed the binaries through gup command
- If the user manually edits the configuration file
  
# How to install
### Step.1 Install golang
gup command only supports installation with `$ go install`. If you does not have the golang development environment installed on your system, please install golang from the [golang official website] (https://go.dev/doc/install).

### Step2. Install gup
```
$ go install github.com/nao1215/gup@latest
```
# How to use
### Update all binaries
If you update all binaries, you just run `$ gup`.  After executing the gup command, a configuration file is automatically created in `$HOME/.config/gup/gup.conf`.

```
$ gup
```

### Add the package path to the config file
gup command can not obtaine all package path information from the shell history. So, you need to add the PATH information manually.  
  
`$HOME/.config/gup/gup.conf` saves the settings in the`$BINARY_NAME = $PATH` format. $PATH may be empty as shown below.
```
cheat = github.com/cheat/cheat/cmd/cheat
dlv = 
dlv-dap = 
ff = 
fyne_demo = 
gal = github.com/nao1215/gal/cmd/gal
```
The simplest way is to write the path to the config file using your favorite editor. Another method is that you use the gup command to install the binaries.
```
$ gup install command_package_path
```
`$ gup install` is a wrapper for `$ go install`. The difference is that there are no options and gup always get the latest version. The biggest difference is that the path information is recorded in the configuration file of the gup command.

# Contact
If you would like to send comments such as "find a bug" or "request for additional features" to the developer, please use one of the following contacts.

- [GitHub Issue](https://github.com/nao1215/gup/issues)

# LICENSE
The gup project is licensed under the terms of the Apache License 2.0.
