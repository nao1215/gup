#shellcheck shell=bash
# End-to-end coverage for `gup migrate`, reinstalling binaries from one GOBIN
# directory into another in an isolated, offline environment.

Describe 'gup migrate'
  BeforeEach 'e2e_setup'
  AfterEach 'e2e_teardown'

  It 'reinstalls binaries from BEFORE_PATH into AFTER_PATH'
    before="$E2E_WORK/work/before"
    after="$E2E_WORK/work/after"
    mkdir -p "$before"
    install_fixture_into "$before" gup.test/outdated@v1.0.0
    When call gup migrate "$before" "$after"
    The status should be success
    The path "$after/outdated" should be exist
    The output should include 'migration'
  End
End
