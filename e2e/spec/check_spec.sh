#shellcheck shell=bash
# End-to-end coverage for `gup check` against the real `go` toolchain and a local
# offline module proxy (#347).

Describe 'gup check'
  BeforeEach 'e2e_setup'
  AfterEach 'e2e_teardown'

  It 'reports up-to-date for a binary already at the latest version'
    install_fixture gup.test/uptodate@v1.0.0
    When call gup check --json
    The status should be success
    The output should include '"name": "uptodate"'
    The output should include '"status": "up-to-date"'
  End

  It 'reports update-available for an outdated binary'
    install_fixture gup.test/outdated@v1.0.0
    When call gup check --json
    The status should be success
    The output should include '"name": "outdated"'
    The output should include '"status": "update-available"'
    The output should include '"latest_version": "v1.1.0"'
  End

  # A file present in $GOBIN whose build info can't be read (here a non-Go junk
  # binary) must be reported as unreadable, NOT as "not found": it exists, it
  # just can't be managed. Regression for the missing-target mislabeling.
  It 'does not mislabel a present-but-unreadable binary as not found'
    printf 'not a go binary' > "$GOBIN/junk"
    When call gup check junk
    The status should be failure
    The stderr should include 'could not read Go build info'
    The stderr should not include "not found 'junk'"
  End
End
