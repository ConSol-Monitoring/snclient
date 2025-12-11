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

    # New-Item -Force behaves like touch/mkdir -p
    New-Item -ItemType File -Path 'file1.txt' -Force
    New-Item -ItemType File -Path 'file2' -Force

    New-Item -ItemType Directory -Path 'directory1' -Force
    New-Item -ItemType File -Path 'directory1/directory1-file3.txt' -Force
    New-Item -ItemType File -Path 'directory1/directory1-file4' -Force

    New-Item -ItemType Directory -Path 'directory1/directory1-directory2' -Force
    New-Item -ItemType File -Path 'directory1/directory1-directory2/directory1-directory2-file5' -Force
    New-Item -ItemType File -Path 'directory1/directory1-directory2/directory1-directory2-file6' -Force
    New-Item -ItemType File -Path 'directory1/directory1-directory2/directory1-directory2-file7' -Force

    New-Item -ItemType Directory -Path 'directory1/directory1-directory2/directory1-directory2-directory3' -Force
    New-Item -ItemType File -Path 'directory1/directory1-directory2/directory1-directory2-directory3/directory1-directory2-directory3-file8' -Force

    New-Item -ItemType Directory -Path 'directory4' -Force
    New-Item -ItemType File -Path 'directory4/directory4-file9.exe' -Force
    New-Item -ItemType File -Path 'directory4/directory4-file10.html' -Force
    New-Item -ItemType Directory -Path 'directory4/directory4-directory5' -Force
    New-Item -ItemType Directory -Path 'directory4/directory4-directory6' -Force
    New-Item -ItemType File -Path 'directory4/directory4-directory5/directory4-directory5-file11' -Force

    $fileCount = (Get-ChildItem -Recurse -File | Measure-Object).Count
    Write-Output "ok - Generated $fileCount files for testing"

    $dirCount = (Get-ChildItem -Recurse -Directory | Measure-Object).Count
    Write-Output "ok - Generated $dirCount directories for testing"

    Write-Output "printing the tree of the files"
    # if tree is available, use it. Otherwise we have to use a for loop
    if (Get-Command tree -ErrorAction SilentlyContinue) {
        tree $TestingDir
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
