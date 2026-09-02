param(
  [ValidateSet('server','workspace','all')]
  [string]$Target = 'server',
  [int]$TailLines = 80
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$logDir = Join-Path $root '.agent-logs'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'

function Invoke-CompactCheck {
  param(
    [string]$Name,
    [string]$WorkingDirectory,
    [string]$Command
  )

  $safeName = ($Name -replace '[^A-Za-z0-9_.-]', '-')
  $logPath = Join-Path $logDir "$stamp-$safeName.log"
  Push-Location $WorkingDirectory
  try {
    cmd.exe /d /s /c "$Command > `"$logPath`" 2>&1"
    $exitCode = $LASTEXITCODE
  }
  finally {
    Pop-Location
  }

  if ($exitCode -eq 0) {
    Write-Host "PASS $Name"
    return
  }

  Write-Host "FAIL $Name (log: $logPath)"
  if (Test-Path $logPath) {
    Get-Content $logPath -Tail $TailLines
  }
  exit $exitCode
}

if ($Target -in @('server','all')) {
  Invoke-CompactCheck 'server-config' (Join-Path $root 'services\server') 'go test ./internal/config -count=1'
  Invoke-CompactCheck 'server-agent-session-rotation' (Join-Path $root 'services\server') 'go test ./internal/service/agent -run "Test(SessionServiceRotatesACPContextAfterBoundedTurns|ShouldRotateACPSessionTreatsUnknownPersistedContextAsOverBudget)$" -count=1'
  Invoke-CompactCheck 'server-acp-recap' (Join-Path $root 'services\server') 'go test ./internal/service/acp -run "Test(BuildACPSessionRecap|ACPSessionRecap)" -count=1'
  Invoke-CompactCheck 'server-build' (Join-Path $root 'services\server') 'go build ./...'
}

if ($Target -in @('workspace','all')) {
  Invoke-CompactCheck 'workspace-build' (Join-Path $root 'apps\workspace') 'pnpm build'
}
