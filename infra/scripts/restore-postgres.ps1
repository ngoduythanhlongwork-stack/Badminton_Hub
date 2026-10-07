param(
    [Parameter(Mandatory = $true)][string]$BackupPath,
    [switch]$ConfirmDatabaseReset,
    [string]$ComposeFile = ".\infra\compose.yaml"
)

$ErrorActionPreference = "Stop"
if (-not $ConfirmDatabaseReset) {
    throw "Restore replaces objects in badminton_hub. Re-run with -ConfirmDatabaseReset."
}
$resolvedBackup = (Resolve-Path -LiteralPath $BackupPath).Path
$resolvedCompose = (Resolve-Path -LiteralPath $ComposeFile).Path
$stderrPath = Join-Path ([System.IO.Path]::GetTempPath()) "badminton-hub-pg-restore-$([Guid]::NewGuid().ToString('N')).err"
$arguments = @("compose", "-f", $resolvedCompose, "exec", "-T", "postgres", "pg_restore", "--username", "badminton_hub", "--dbname", "badminton_hub", "--clean", "--if-exists", "--no-owner", "--no-acl")
$process = Start-Process -FilePath "docker" -ArgumentList $arguments -NoNewWindow -Wait -PassThru -RedirectStandardInput $resolvedBackup -RedirectStandardError $stderrPath
if ($process.ExitCode -ne 0) {
    throw "pg_restore failed. See $stderrPath"
}
Write-Output "Restore completed from $resolvedBackup"
