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

    New-Item -ItemType Directory -Path "dir1" -Force > $null

    New-Item -ItemType File -Path "dir1/file1.txt" -Force > $null
    $string = "This is a test line.`r`n"
    $content = $string * 100
    Set-Content -Path "dir1/file1.txt" -Value $content -NoNewline

    # Symbolic link to folder
    cmd /c mklink /d "dir1_symlink1" "dir1"

    # Symbolic link to file
    cmd /c mklink "file1_symlink1.txt" "dir1\file1.txt"

    # Hard link to file
    cmd /c mklink /h "file1_hardlink1.txt" "dir1\file1.txt"

    # Junction to folder
    cmd /c mklink /j "dir1_junction1" "dir1"

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
