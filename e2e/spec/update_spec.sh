#shellcheck shell=bash
# End-to-end coverage for `gup update` against the real `go` toolchain and a local
# offline module proxy (#347), including the @main -> @master fallback rules.

Describe 'gup update'
  BeforeEach 'e2e_setup'
  AfterEach 'e2e_teardown'

  It 'updates an outdated binary'
    install_fixture gup.test/outdated@v1.0.0
    When call gup update outdated
    The status should be success
    The output should include 'outdated'
    The path "$GOBIN/outdated" should be exist
  End

  It 'installs the newer version (binary build info reflects v1.1.0)'
    install_fixture gup.test/outdated@v1.0.0
    gup update outdated >/dev/null 2>&1
    When call go version -m "$GOBIN/outdated"
    The output should include 'gup.test/outdated'
    The output should include 'v1.1.0'
  End

  Describe '--main / @master fallback'
    It 'installs from @main when the main branch exists'
      install_fixture gup.test/maintool@main
      When call gup update --main maintool
      The status should be success
      The output should include 'maintool'
      The path "$GOBIN/maintool" should be exist
    End

    It 'falls back to @master only when the repo has no main branch'
      install_fixture gup.test/mastertool@master
      When call gup update --main mastertool
      The status should be success
      The output should include 'mastertool'
      The path "$GOBIN/mastertool" should be exist
    End

    It 'does NOT fall back to @master when @main exists but fails to build'
      install_fixture gup.test/badmaintool@v1.0.0
      When call gup update --main badmaintool
      The status should be failure
      The output should include 'update binary under'
      # The build failure is reported; @master is never tried as a fallback (the
      # "leaves the binary unchanged" example below proves @master was not used).
      The stderr should not include 'badmaintool master'
    End

    It 'leaves the binary unchanged when @main fails to build (no @master fallback)'
      install_fixture gup.test/badmaintool@v1.0.0
      gup update --main badmaintool >/dev/null 2>&1 || true
      When call go version -m "$GOBIN/badmaintool"
      The output should include 'v1.0.0'
    End
  End
End
