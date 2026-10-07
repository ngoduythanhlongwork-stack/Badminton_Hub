param(
    [string]$OutputDirectory = ".\backups",
    [string]$ComposeFile = ".\infra\compose.yaml"
)

$ErrorActionPreference = "Stop"
$resolvedCompose = (Resolve-Path -LiteralPath $ComposeFile).Path
$resolvedOutput = [System.IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $resolvedOutput -Force | Out-Null
$timestamp = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
$backupPath = Join-Path $resolvedOutput "badminton_hub-$timestamp.dump"
$stderrPath = Join-Path ([System.IO.Path]::GetTempPath()) "badminton-hub-pg-dump-$timestamp.err"

$arguments = @("compose", "-f", $resolvedCompose, "exec", "-T", "postgres", "pg_dump", "--username", "badminton_hub", "--dbname", "badminton_hub", "--format", "custom", "--no-owner", "--no-acl")
$process = Start-Process -FilePath "docker" -ArgumentList $arguments -NoNewWindow -Wait -PassThru -RedirectStandardOutput $backupPath -RedirectStandardError $stderrPath
if ($process.ExitCode -ne 0) {
    throw "pg_dump failed. See $stderrPath"
}
if ((Get-Item -LiteralPath $backupPath).Length -eq 0) {
    throw "pg_dump produced an empty backup: $backupPath"
}
Write-Output $backupPath
