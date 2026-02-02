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

    # Create files of 512KB (0.5MB)
    $data = New-Object byte[] (1024 * 512)
    $fileNames = @(
        "file_512kb_1.root",
        "file_512kb_2.root", 
        "file_512kb_3.root", 
        "file_512kb_4.root"
    )
    foreach ($fileName in $fileNames) {
        New-Item -ItemType File -Path $fileName -Value "test" > $null
        [System.IO.File]::WriteAllBytes((Get-Item $fileName).FullName, $data)
    }

    $data = New-Object byte[] (1024 * 1024)

    # Create files of 1024KB (1MB)
    $fileNames = @(
        "file_1024kb_1.root",
        "file_1024kb_2.root", 
        "file_1024kb_3.root"
    )
    foreach ($fileName in $fileNames) {
        New-Item -ItemType File -Path $fileName -Value "test" > $null
        [System.IO.File]::WriteAllBytes((Get-Item $fileName).FullName, $data)
    }

    New-Item -ItemType Directory -Path "a" -Force > $null
    $fileNames = @(
        "a/file_1024kb_1.a",
        "a/file_1024kb_2.a", 
        "a/file_1024kb_3.a",
        "a/file_1024kb_4.a"
    )
    foreach ($fileName in $fileNames) {
        New-Item -ItemType File -Path $fileName -Value "test" > $null
        [System.IO.File]::WriteAllBytes((Get-Item $fileName).FullName, $data)
    }

    New-Item -ItemType Directory -Path "b" -Force > $null
    $fileNames = @(
        "b/file_1024kb_1.b",
        "b/file_1024kb_2.b", 
        "b/file_1024kb_3.b",
        "b/file_1024kb_4.b",
        "b/file_1024kb_5.b"
    )
    foreach ($fileName in $fileNames) {
        New-Item -ItemType File -Path $fileName -Value "test" > $null
        [System.IO.File]::WriteAllBytes((Get-Item $fileName).FullName, $data)
    }

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
