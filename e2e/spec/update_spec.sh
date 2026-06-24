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

  Describe 'channel persistence destination'
    # Regression for the bug where an explicit --file was ignored as the write
    # destination unless the file already existed, so the channel was silently
    # saved to the user-level gup.json instead.
    It 'saves the channel to an explicit --file even when the file does not exist yet'
      install_fixture gup.test/maintool@main
      explicit="$E2E_WORK/work/explicit.json"
      When call gup update --main maintool --file "$explicit"
      The status should be success
      The output should include 'maintool'
      The path "$explicit" should be exist
      The contents of file "$explicit" should include '"channel": "main"'
      # The user-level config must NOT be created as a side effect.
      The path "$XDG_CONFIG_HOME/gup/gup.json" should not be exist
    End
  End

  Describe 'ambiguous config'
    # Regression for #342: when both the user-level config and ./gup.json exist
    # and no --file is given, update must fail fast instead of silently choosing.
    It 'fails fast when both the user-level config and ./gup.json exist'
      install_fixture gup.test/uptodate@v1.0.0
      mkdir -p "$XDG_CONFIG_HOME/gup"
      printf '%s\n' '{"schema_version":1,"packages":[{"name":"uptodate","import_path":"gup.test/uptodate","version":"v1.0.0","channel":"latest"}]}' > "$XDG_CONFIG_HOME/gup/gup.json"
      printf '%s\n' '{"schema_version":1,"packages":[{"name":"uptodate","import_path":"gup.test/uptodate","version":"v1.0.0","channel":"latest"}]}' > "$E2E_WORK/work/gup.json"
      When call gup update
      The status should be failure
      The stderr should include 'multiple gup.json'
      The stderr should include '--file'
    End
  End

  Describe 'failure diagnostics / next-step hints'
    # gup.test/moved ships its command under cmd/tool at v1.0.0, but the newer
    # @latest (v1.1.0) no longer contains that package, so the real go toolchain
    # fails with "found (v1.1.0), but does not contain package ...". This is the
    # realistic "the tool moved (e.g. /v2 bump)" failure; gup must turn that
    # cryptic output into an actionable next step.
    It 'prints a next-step hint when the command path moved away'
      install_fixture gup.test/moved/cmd/tool@v1.0.0
      When call gup update tool
      The status should be failure
      The output should include 'update binary under'
      # The raw toolchain error is still surfaced...
      The stderr should include 'does not contain package'
      # ...followed by the actionable hint.
      The stderr should include 'gup:'
      The stderr should include 'major version'
    End

    # gup.test/replaced installs cleanly at v1.0.0, but its newer @latest
    # (v1.1.0) adds a replace directive to go.mod, so the real go toolchain
    # refuses it with "contains one or more replace directives" — the one
    # diagnostic class go-global-update documented (E004) that gup lacked.
    It 'prints a next-step hint when the module uses replace directives'
      install_fixture gup.test/replaced@v1.0.0
      When call gup update replaced
      The status should be failure
      The output should include 'update binary under'
      The stderr should include 'replace directive'
      The stderr should include 'go install'
    End
  End
End
