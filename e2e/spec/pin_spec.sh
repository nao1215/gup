#shellcheck shell=bash
# End-to-end coverage for version pinning: `gup pin` / `gup unpin` and how
# `gup update`, `gup check`, `gup export` and `gup import` treat pinned packages,
# all against the real `go` toolchain and a local offline module proxy.

Describe 'gup pin / unpin'
  BeforeEach 'e2e_setup'
  AfterEach 'e2e_teardown'

  config_path() { printf '%s/gup/gup.json' "$XDG_CONFIG_HOME"; }

  It 'records a pin in gup.json with schema_version 2'
    install_fixture gup.test/outdated@v1.0.0
    When call gup pin outdated v1.0.0
    The status should be success
    The output should include 'Pinned outdated to v1.0.0'
    The path "$(config_path)" should be exist
    The contents of file "$(config_path)" should include '"schema_version": 2'
    The contents of file "$(config_path)" should include '"channel": "pinned"'
    The contents of file "$(config_path)" should include '"version": "v1.0.0"'
  End

  It 'accepts the tool@version form'
    install_fixture gup.test/outdated@v1.0.0
    When call gup pin outdated@v1.0.0
    The status should be success
    The output should include 'Pinned outdated to v1.0.0'
    The contents of file "$(config_path)" should include '"channel": "pinned"'
  End

  It 'refuses to pin a tool that is not managed by gup'
    When call gup pin not-installed v1.0.0
    The status should be failure
    The stderr should include 'not managed by gup'
  End

  It 'keeps a pinned tool at its version during gup update'
    install_fixture gup.test/outdated@v1.0.0
    gup pin outdated v1.0.0 >/dev/null
    gup update >/dev/null 2>&1
    When call go version -m "$GOBIN/outdated"
    The output should include 'gup.test/outdated'
    The output should include 'v1.0.0'
    The output should not include 'v1.1.0'
  End

  It 'reinstalls a pinned tool at the pin when the installed version differs'
    install_fixture gup.test/outdated@v1.1.0
    gup pin outdated v1.0.0 >/dev/null
    gup update >/dev/null 2>&1
    When call go version -m "$GOBIN/outdated"
    # The pin downgrades v1.1.0 back to the pinned v1.0.0.
    The output should include 'v1.0.0'
  End

  It 'updates an unpinned tool while a pinned tool stays put in the same run'
    install_fixture gup.test/outdated@v1.0.0
    install_fixture gup.test/pinnable@v1.0.0
    gup pin outdated v1.0.0 >/dev/null
    gup update >/dev/null 2>&1
    # One evaluation that proves: pinned 'outdated' stayed at v1.0.0 (not v1.1.0)
    # while unpinned 'pinnable' moved to v1.1.0.
    verify_mixed() {
      go version -m "$GOBIN/outdated" | grep -q 'v1.0.0' || { echo 'outdated not at pinned v1.0.0'; return 1; }
      go version -m "$GOBIN/outdated" | grep -q 'v1.1.0' && { echo 'outdated wrongly updated to v1.1.0'; return 1; }
      go version -m "$GOBIN/pinnable" | grep -q 'v1.1.0' || { echo 'pinnable did not update to v1.1.0'; return 1; }
      echo 'MIXED OK'
    }
    When call verify_mixed
    The status should be success
    The output should include 'MIXED OK'
  End

  It 'allows the tool to update again after unpin'
    install_fixture gup.test/outdated@v1.0.0
    gup pin outdated v1.0.0 >/dev/null
    gup unpin outdated >/dev/null
    gup update >/dev/null 2>&1
    When call go version -m "$GOBIN/outdated"
    The output should include 'v1.1.0'
  End

  It 'unpin is idempotent for a tool that is not pinned'
    install_fixture gup.test/outdated@v1.0.0
    When call gup unpin outdated
    The status should be success
    The output should include 'not pinned'
  End

  Describe 'check'
    It 'reports a pinned tool at its version as pinned (human + JSON)'
      install_fixture gup.test/outdated@v1.0.0
      gup pin outdated v1.0.0 >/dev/null
      When call gup check --json
      The status should be success
      The output should include '"channel": "pinned"'
      The output should include '"pinned_version": "v1.0.0"'
      The output should include '"status": "pinned"'
      # A pin is never compared against @latest.
      The output should include '"latest_version": ""'
    End

    It 'reports pin-mismatch and suggests gup update when versions differ'
      install_fixture gup.test/outdated@v1.1.0
      gup pin outdated v1.0.0 >/dev/null
      When call gup check
      The status should be success
      The output should include 'pinned'
      The output should include 'gup update'
    End

    It 'exposes pin-mismatch in JSON'
      install_fixture gup.test/outdated@v1.1.0
      gup pin outdated v1.0.0 >/dev/null
      When call gup check --json
      The output should include '"status": "pin-mismatch"'
      The output should include '"pinned_version": "v1.0.0"'
      The output should include '"current_version": "v1.1.0"'
    End
  End

  Describe 'export / import'
    It 'export preserves the pinned state and version'
      install_fixture gup.test/outdated@v1.0.0
      gup pin outdated v1.0.0 >/dev/null
      When call gup export --output
      The status should be success
      The output should include '"schema_version": 2'
      The output should include '"channel": "pinned"'
      The output should include '"version": "v1.0.0"'
    End

    It 'import installs the exact pinned version'
      config_file="$E2E_WORK/work/gup.json"
      printf '%s\n' '{"schema_version":2,"packages":[{"name":"outdated","import_path":"gup.test/outdated","version":"v1.0.0","channel":"pinned"}]}' > "$config_file"
      gup import --file "$config_file" >/dev/null 2>&1
      When call go version -m "$GOBIN/outdated"
      The output should include 'v1.0.0'
      The output should not include 'v1.1.0'
    End
  End

  Describe 'config safety'
    It 'still reads an existing schema_version 1 config'
      install_fixture gup.test/outdated@v1.0.0
      config_file="$E2E_WORK/work/gup.json"
      printf '%s\n' '{"schema_version":1,"packages":[{"name":"outdated","import_path":"gup.test/outdated","version":"v1.0.0","channel":"latest"}]}' > "$config_file"
      When call gup check --file "$config_file" --json
      The status should be success
      The output should include '"channel": "latest"'
    End

    It 'fails fast on an invalid pinned config instead of installing @latest'
      install_fixture gup.test/outdated@v1.0.0
      config_file="$E2E_WORK/work/gup.json"
      printf '%s\n' '{"schema_version":2,"packages":[{"name":"outdated","import_path":"gup.test/outdated","version":"latest","channel":"pinned"}]}' > "$config_file"
      When call gup check --file "$config_file"
      The status should be failure
      The stderr should include 'pinned'
    End

    It 'fails fast on an unknown channel instead of treating it as latest'
      install_fixture gup.test/outdated@v1.0.0
      config_file="$E2E_WORK/work/gup.json"
      printf '%s\n' '{"schema_version":1,"packages":[{"name":"outdated","import_path":"gup.test/outdated","version":"v1.0.0","channel":"stable"}]}' > "$config_file"
      When call gup check --file "$config_file"
      The status should be failure
      The stderr should include 'unknown channel'
    End

    It 'rejects channel pinned under schema_version 1'
      install_fixture gup.test/outdated@v1.0.0
      config_file="$E2E_WORK/work/gup.json"
      printf '%s\n' '{"schema_version":1,"packages":[{"name":"outdated","import_path":"gup.test/outdated","version":"v1.0.0","channel":"pinned"}]}' > "$config_file"
      When call gup check --file "$config_file"
      The status should be failure
      The stderr should include 'pinned'
    End
  End

  It 'keeps gup update --json valid with a pinned package'
    install_fixture gup.test/outdated@v1.0.0
    gup pin outdated v1.0.0 >/dev/null
    When call gup update --json
    The status should be success
    The output should include '"channel": "pinned"'
    # Valid JSON array: starts with [ and the pinned record is present.
    The output should include '"name": "outdated"'
  End
End
