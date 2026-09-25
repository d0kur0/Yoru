param([switch]$SkipInstall)

$ErrorActionPreference = 'Stop'

function Find-NSIS {
    $command = Get-Command makensis.exe -CommandType Application -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    # NSIS packages may install under either Program Files directory.
    foreach ($root in @($env:ProgramFiles, ${env:ProgramFiles(x86)})) {
        if (-not $root) { continue }
        $candidate = Join-Path $root 'NSIS\makensis.exe'
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
    }
    return $null
}

$nsisExecutable = Find-NSIS
if (-not $nsisExecutable -and -not $SkipInstall) {
    # Chocolatey may report success even when its package feed returned 504.
    # Check the executable after every attempt, not only the command exit code.
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        choco install nsis --yes --no-progress --force
        $installExitCode = $LASTEXITCODE
        $nsisExecutable = Find-NSIS
        if ($nsisExecutable) { break }
        if ($attempt -lt 3) {
            Write-Warning "NSIS is still missing after attempt $attempt (Chocolatey exit $installExitCode); retrying."
            Start-Sleep -Seconds (10 * $attempt)
        }
    }
}
if (-not $nsisExecutable) { throw 'NSIS installation did not provide makensis.exe in PATH or either Program Files directory.' }

$nsisDirectory = Split-Path -Parent $nsisExecutable
$env:PATH = "$nsisDirectory;$env:PATH"
if ($env:GITHUB_PATH) {
    $nsisDirectory | Out-File -LiteralPath $env:GITHUB_PATH -Encoding utf8 -Append
}
Write-Host "NSIS executable: $nsisExecutable"
& $nsisExecutable /VERSION
if ($LASTEXITCODE -ne 0) { throw "makensis verification failed: $LASTEXITCODE" }
