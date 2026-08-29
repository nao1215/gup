package cmd

// Subcommand names, in one place.
//
// They were repeated as literals across the command constructors, the lock
// policy, and the tests, which is how a rename ends up applied in four places
// and missed in a fifth. commandLockPolicy keys off these, so a typo there is a
// compile error rather than a command that silently runs unlocked.
const (
	cmdNameUpdate     = "update"
	cmdNameImport     = "import"
	cmdNameExport     = "export"
	cmdNameRemove     = "remove"
	cmdNameMigrate    = "migrate"
	cmdNamePin        = "pin"
	cmdNameUnpin      = "unpin"
	cmdNameCheck      = "check"
	cmdNameList       = "list"
	cmdNameVersion    = "version"
	cmdNameCompletion = "completion"
	cmdNameMan        = "man"
	cmdNameBugReport  = "bug-report"
)
