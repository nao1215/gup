#shellcheck shell=bash
# End-to-end coverage for `gup remove`, including the non-TTY safety behavior, in
# an isolated, offline environment.

Describe 'gup remove'
  BeforeEach 'e2e_setup'
  AfterEach 'e2e_teardown'

  It 'removes an installed binary with --force'
    install_fixture gup.test/outdated@v1.0.0
    When call gup_no_tty remove --force outdated
    The status should be success
    The output should include 'removed'
    The path "$GOBIN/outdated" should not be exist
  End

  It 'refuses to remove without --force when stdin is not a TTY (#323)'
    install_fixture gup.test/outdated@v1.0.0
    When call gup_no_tty remove outdated
    The status should be failure
    The stderr should include 'not a TTY'
    The path "$GOBIN/outdated" should be exist
  End
End
