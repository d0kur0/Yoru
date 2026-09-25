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
    choco install nsis --yes --no-progress
    if ($LASTEXITCODE -ne 0) { throw "NSIS installation failed: $LASTEXITCODE" }
    $nsisExecutable = Find-NSIS
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
