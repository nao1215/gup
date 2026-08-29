<#
.SYNOPSIS
    Runs the Windows release archive and checks that the shipped gup works.

.DESCRIPTION
    scripts/smoke_artifacts.sh inspects every artifact from Ubuntu -- contents,
    recorded GOOS/GOARCH, package file trees, manifest hashes -- but it cannot
    execute the Windows binary, and "the zip contains something that says it is a
    Windows binary" is a weaker claim than "the Windows binary runs". This script
    is the other half: it unpacks the zip on a real Windows runner and exercises
    the CLI surface a user and a CI script actually depend on.

    It is PowerShell rather than bash on purpose. The Windows legs of gup's CI
    must not depend on Git Bash being installed and on PATH: a check that only
    passes because the runner image happens to ship a POSIX shell is testing the
    image, not gup.

.PARAMETER DistDir
    Directory holding the GoReleaser output. Defaults to "dist".

.EXAMPLE
    pwsh -File scripts/verify_artifact.ps1 -DistDir dist
#>
[CmdletBinding()]
param(
    [string]$DistDir = 'dist'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Note { param([string]$Message) Write-Host "smoke(windows): $Message" }

function Stop-WithFailure {
    param([string]$Message)
    Write-Host "smoke(windows): FAIL: $Message"
    exit 1
}

if (-not (Test-Path -LiteralPath $DistDir -PathType Container)) {
    Stop-WithFailure "dist directory '$DistDir' does not exist (run goreleaser first)"
}

# GitHub's Windows runners are x64 today and Arm64 images exist, so the
# architecture is read rather than assumed: running the wrong archive would
# either fail confusingly or, under emulation, quietly pass while testing the
# other artifact.
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { $null }
}
if (-not $arch) {
    Stop-WithFailure "unrecognized processor architecture '$($env:PROCESSOR_ARCHITECTURE)'"
}
Write-Note "host: windows/$arch, dist: $DistDir"

$zip = Get-ChildItem -LiteralPath $DistDir -Filter "*_windows_$arch.zip" | Select-Object -First 1
if (-not $zip) {
    Stop-WithFailure "no windows/$arch archive (*_windows_$arch.zip) in $DistDir"
}

$workdir = Join-Path ([System.IO.Path]::GetTempPath()) ("gup-smoke-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $workdir -Force | Out-Null
try {
    Expand-Archive -LiteralPath $zip.FullName -DestinationPath $workdir -Force
    Write-Note "extracted $($zip.Name)"

    $gup = Join-Path $workdir 'gup.exe'
    if (-not (Test-Path -LiteralPath $gup -PathType Leaf)) {
        Stop-WithFailure "$($zip.Name) does not contain gup.exe"
    }

    # The completion files ride along in the archive; a Windows user installing
    # from the zip has no package manager to place them, so their absence is a
    # silently degraded install rather than a build failure.
    foreach ($shell in @('bash', 'zsh', 'fish', 'ps1')) {
        $completion = Join-Path $workdir "completions/gup.$shell"
        if (-not (Test-Path -LiteralPath $completion -PathType Leaf)) {
            Stop-WithFailure "$($zip.Name) is missing completions/gup.$shell"
        }
    }
    Write-Note 'OK   the archive carries gup.exe and the bash/zsh/fish/powershell completions'

    $version = & $gup --version
    if ($LASTEXITCODE -ne 0) { Stop-WithFailure "gup --version exited $LASTEXITCODE" }
    if ($version -notmatch 'gup version') {
        Stop-WithFailure "gup --version output unexpected: $version"
    }
    Write-Note "OK   gup --version runs: $version"

    $help = & $gup --help
    if ($LASTEXITCODE -ne 0) { Stop-WithFailure "gup --help exited $LASTEXITCODE" }
    if (($help -join "`n") -notmatch 'Available Commands') {
        Stop-WithFailure 'gup --help did not list the subcommands'
    }
    Write-Note 'OK   gup --help lists the subcommands'

    # An empty $GOBIN keeps the JSON checks hermetic: no network, and no
    # dependence on what the runner image happens to have installed.
    $emptyGobin = Join-Path $workdir 'empty-gobin'
    New-Item -ItemType Directory -Path $emptyGobin -Force | Out-Null
    $env:GOBIN = $emptyGobin

    foreach ($subcommand in @('list', 'check')) {
        $raw = (& $gup $subcommand --json) -join "`n"
        if ($LASTEXITCODE -ne 0) { Stop-WithFailure "gup $subcommand --json exited $LASTEXITCODE" }
        try {
            # -NoEnumerate keeps an empty array an array instead of $null, which
            # is exactly the case an empty $GOBIN produces.
            $parsed = ConvertFrom-Json -InputObject $raw -NoEnumerate
        } catch {
            Stop-WithFailure "gup $subcommand --json produced invalid JSON: $raw"
        }
        if ($parsed -isnot [System.Array]) {
            Stop-WithFailure "gup $subcommand --json did not produce a JSON array: $raw"
        }
        Write-Note "OK   gup $subcommand --json produces a valid JSON array"
    }

    # Completion install is the one command whose Windows behavior differs from
    # POSIX, so the shipped binary is made to perform it against a throwaway
    # profile: this is the path a Windows user takes right after installing.
    $profileHome = Join-Path $workdir 'profile-home'
    New-Item -ItemType Directory -Path $profileHome -Force | Out-Null
    $env:PROFILE = Join-Path $profileHome 'Microsoft.PowerShell_profile.ps1'
    & $gup completion --install
    if ($LASTEXITCODE -ne 0) { Stop-WithFailure "gup completion --install exited $LASTEXITCODE" }
    if (-not (Test-Path -LiteralPath $env:PROFILE -PathType Leaf)) {
        Stop-WithFailure 'gup completion --install did not create the PowerShell profile'
    }
    $completionScript = Join-Path $profileHome 'gup.completion.ps1'
    if (-not (Test-Path -LiteralPath $completionScript -PathType Leaf)) {
        Stop-WithFailure 'gup completion --install did not write gup.completion.ps1'
    }
    if ((Get-Content -Raw -LiteralPath $env:PROFILE) -notmatch 'setting for gup command') {
        Stop-WithFailure 'gup completion --install did not add its block to the profile'
    }
    # The generated completer has to be loadable, or completion fails at every
    # prompt with a parse error the user cannot act on.
    . $completionScript
    Write-Note 'OK   gup completion --install wires up a loadable PowerShell completer'

    Write-Host ''
    Write-Note "===== verification scope on windows/$arch ====="
    Write-Note '  verified: the windows archive extracts and gup.exe runs (--version, --help)'
    Write-Note '  verified: list --json and check --json produce valid JSON arrays'
    Write-Note '  verified: completion --install writes a loadable PowerShell completer and profile block'
    Write-Note "  not verified here: the windows/$(if ($arch -eq 'amd64') { 'arm64' } else { 'amd64' }) binary -- no runner for it; its GOOS/GOARCH is verified by smoke_artifacts.sh"
    Write-Note '  not verified here: the linux and darwin artifacts -- covered by the ubuntu and macos smoke jobs'
    Write-Note 'all Windows artifact smoke checks passed'
} finally {
    Remove-Item -LiteralPath $workdir -Recurse -Force -ErrorAction SilentlyContinue
}
