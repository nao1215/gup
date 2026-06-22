#shellcheck shell=bash
# End-to-end coverage for `gup export` and `gup import` round-tripping through a
# gup.json in an isolated, offline environment.

Describe 'gup export / import'
  BeforeEach 'e2e_setup'
  AfterEach 'e2e_teardown'

  It 'export --output prints the installed tool as JSON on STDOUT'
    install_fixture gup.test/outdated@v1.0.0
    When call gup export --output
    The status should be success
    The output should include 'gup.test/outdated'
    The output should include '"schema_version"'
  End

  It 'import --file installs the tools listed in a gup.json'
    config_file="$E2E_WORK/work/gup.json"
    printf '%s\n' '{"schema_version":1,"packages":[{"name":"uptodate","import_path":"gup.test/uptodate","version":"v1.0.0","channel":"latest"}]}' > "$config_file"
    When call gup import --file "$config_file"
    The status should be success
    The path "$GOBIN/uptodate" should be exist
    The output should include 'gup.test/uptodate'
  End
End
