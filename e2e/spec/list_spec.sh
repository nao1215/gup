#shellcheck shell=bash
# End-to-end coverage for `gup list` against the real CLI in an isolated,
# offline environment.

Describe 'gup list'
  BeforeEach 'e2e_setup'
  AfterEach 'e2e_teardown'

  It 'lists an installed binary with its import path'
    install_fixture gup.test/outdated@v1.0.0
    When call gup list
    The status should be success
    The output should include 'outdated'
    The output should include 'gup.test/outdated'
  End

  It 'reports a friendly note on an empty environment (#350)'
    When call gup list
    The status should be success
    The output should include 'no binaries are installed'
  End

  It 'emits a valid empty JSON array on an empty environment with --json'
    When call gup list --json
    The status should be success
    The output should equal '[]'
  End
End
