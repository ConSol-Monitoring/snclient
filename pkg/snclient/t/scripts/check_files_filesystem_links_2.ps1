#!/usr/bin/env pwsh

param(
    [Parameter(Mandatory=$true)]
    [string]$TestingDir
)

$ErrorActionPreference = 'Stop'

New-Item -ItemType Directory -Path $TestingDir -Force

Push-Location -Path $TestingDir
try {
    # Since the new location is pushed on to the stack, it is automatically applied.


    New-Item -ItemType Directory -Name A

    New-Item -ItemType File -Path "A/file.txt" -Force > $null
    $string = "This is a test line.`r`n"
    $content = $string * 100
    Set-Content -Path "A/file.txt" -Value $content -NoNewline

    New-Item -ItemType Directory -Name B

    # Set up recursive links between paths
    New-Item -ItemType Junction -Path .\A\toB -Target (Resolve-Path .\B)
    New-Item -ItemType Junction -Path .\B\toA -Target (Resolve-Path .\A)

    $fileCount = (Get-ChildItem -Recurse -File | Measure-Object).Count
    Write-Output "ok - Generated $fileCount files for testing"

    $dirCount = (Get-ChildItem -Recurse -Directory | Measure-Object).Count
    Write-Output "ok - Generated $dirCount directories for testing"

    Write-Output "printing the tree of the files"
    # if tree is available, use it. Otherwise we have to use a for loop
    if (Get-Command tree -ErrorAction SilentlyContinue) {
        tree /f .
    } else {
        Get-ChildItem -Recurse | Sort-Object FullName | ForEach-Object {
            $relativePath = $_.FullName.Substring($PWD.Path.Length).TrimStart([io.path]::DirectorySeparatorChar)
            Write-Output $relativePath
        }
    }
}
finally {
    Pop-Location
}

exit 0