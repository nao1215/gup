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
End
