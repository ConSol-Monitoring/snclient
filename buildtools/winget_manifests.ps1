[CmdletBinding()]
param(
    [string]$PackageVersion,
    [string]$TagVersion,
    [string]$ReleaseDate,
    [string]$OutDir,
    [string]$TemplateDir,
    [string]$MsiX64,
    [string]$MsiX86,
    [string]$MsiArm64,
    [string]$Sha256X64,
    [string]$Sha256X86,
    [string]$Sha256Arm64,
    [string]$ProductCodeX64,
    [string]$ProductCodeX86,
    [string]$ProductCodeArm64
)

# Generate WinGet manifests from the templates.
# This is the PowerShell counterpart of buildtools/winget_manifests.sh.
# Keep both scripts in sync.
# TAG_VERSION is the release tag without the leading "v".
# PACKAGE_VERSION is the MSI ProductVersion and ARP DisplayVersion.
# WinGet PackageVersion must match the installed MSI version.
# Installer URLs use TAG_VERSION.
# PackageVersion uses PACKAGE_VERSION.

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
if (-not $TemplateDir) {
    $TemplateDir = Join-Path $RepoRoot 'packaging/winget/templates'
}
if (-not $ReleaseDate) {
    $ReleaseDate = Get-Date -Format 'yyyy-MM-dd'
}

function Get-MsiMetadata {
    param([string]$Path)

    $fullPath = (Resolve-Path -LiteralPath $Path).Path
    $installer = New-Object -ComObject WindowsInstaller.Installer
    $database = $installer.OpenDatabase($fullPath, 0)
    $view = $database.OpenView("SELECT Property, Value FROM Property")
    $view.Execute()
    $properties = @{}
    try {
        while ($record = $view.Fetch()) {
            $properties[$record.StringData(1)] = $record.StringData(2)
            [void][Runtime.InteropServices.Marshal]::ReleaseComObject($record)
        }
    } finally {
        $view.Close()
        [void][Runtime.InteropServices.Marshal]::ReleaseComObject($view)
        [void][Runtime.InteropServices.Marshal]::ReleaseComObject($database)
        [void][Runtime.InteropServices.Marshal]::ReleaseComObject($installer)
    }

    [pscustomobject]@{
        ProductCode = $properties['ProductCode']
        ProductVersion = $properties['ProductVersion']
        Sha256 = (Get-FileHash -LiteralPath $fullPath -Algorithm SHA256).Hash
    }
}

$msiData = @{}
if ($MsiX64) {
    $msiData['X64'] = Get-MsiMetadata -Path $MsiX64
    $Sha256X64 = $msiData['X64'].Sha256
    $ProductCodeX64 = $msiData['X64'].ProductCode
}
if ($MsiX86) {
    $msiData['X86'] = Get-MsiMetadata -Path $MsiX86
    $Sha256X86 = $msiData['X86'].Sha256
    $ProductCodeX86 = $msiData['X86'].ProductCode
}
if ($MsiArm64) {
    $msiData['ARM64'] = Get-MsiMetadata -Path $MsiArm64
    $Sha256Arm64 = $msiData['ARM64'].Sha256
    $ProductCodeArm64 = $msiData['ARM64'].ProductCode
}

$msiVersions = @($msiData.Values | ForEach-Object { $_.ProductVersion } | Select-Object -Unique)
if ($msiVersions.Count -gt 1) {
    throw "MSI versions differ: $($msiVersions -join ', ')"
}
if (-not $PackageVersion) {
    if ($msiVersions.Count -eq 1) {
        $PackageVersion = $msiVersions[0]
    } else {
        throw 'PackageVersion is required when no MSI path is given.'
    }
}
if ($msiVersions.Count -eq 1 -and $PackageVersion -ne $msiVersions[0]) {
    throw "PackageVersion $PackageVersion does not match MSI version $($msiVersions[0])."
}
if (-not $TagVersion) {
    throw 'TagVersion is required.'
}

foreach ($value in @($Sha256X64, $Sha256X86, $Sha256Arm64)) {
    if (-not $value) {
        throw 'All SHA256 values are required.'
    }
    if ($value -notmatch '^[0-9A-Fa-f]{64}$') {
        throw "Invalid SHA256 value: $value"
    }
}
foreach ($value in @($ProductCodeX64, $ProductCodeX86, $ProductCodeArm64)) {
    if (-not $value) {
        throw 'All ProductCode values are required.'
    }
    if ($value -notmatch '^\{[0-9A-Fa-f-]+\}$') {
        throw "Invalid ProductCode value: $value"
    }
}
if ($PackageVersion -notmatch '^\d+\.\d+\.\d+$') {
    throw "Invalid package version: $PackageVersion"
}
if ($TagVersion -notmatch '^\d+\.\d+(\.\d+)?$') {
    throw "Invalid tag version: $TagVersion"
}
if ($ReleaseDate -notmatch '^\d{4}-\d{2}-\d{2}$') {
    throw "Invalid release date: $ReleaseDate"
}
if (-not $OutDir) {
    $OutDir = Join-Path $RepoRoot "packaging/winget/$PackageVersion"
}

$replacements = [ordered]@{
    '__PACKAGE_VERSION__' = $PackageVersion
    '__TAG_VERSION__' = $TagVersion
    '__RELEASE_DATE__' = $ReleaseDate
    '__SHA256_X64__' = $Sha256X64
    '__SHA256_X86__' = $Sha256X86
    '__SHA256_ARM64__' = $Sha256Arm64
    '__PRODUCT_CODE_X64__' = $ProductCodeX64
    '__PRODUCT_CODE_X86__' = $ProductCodeX86
    '__PRODUCT_CODE_ARM64__' = $ProductCodeArm64
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$templates = Get-ChildItem -LiteralPath $TemplateDir -Filter '*.yaml.in'
if ($templates.Count -eq 0) {
    throw "No templates found in $TemplateDir"
}
$utf8 = New-Object System.Text.UTF8Encoding($false)
foreach ($template in $templates) {
    $content = Get-Content -LiteralPath $template.FullName -Raw
    foreach ($key in $replacements.Keys) {
        $content = $content.Replace($key, $replacements[$key])
    }
    if ($content -match '__[A-Z0-9_]+__') {
        throw "Unresolved placeholder in $($template.Name)"
    }
    $output = Join-Path $OutDir ($template.Name -replace '\.in$', '')
    [IO.File]::WriteAllText($output, $content, $utf8)
    Write-Output "generated $output"
}
