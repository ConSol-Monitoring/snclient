# check for windows updates
# usage: .\check_os_updates.ps1 [ -Online ]
#
# docs:
# https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nf-wuapi-iupdatesearcher-search
# https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iupdate
#
param (
    [switch]$Online
)
if ($env:ONLINE_SEARCH -eq "1") {
    $Online = $true
}

$update = new-object -com Microsoft.update.Session
if ($update -eq $null) { Write-Host "failed to get Microsoft.update.Session"; exit 1; }

$searcher = $update.CreateUpdateSearcher()
if ($searcher -eq $null) { Write-Host "failed to create update searcher"; exit 1; }

$searcher.Online = 0
if ($Online) {
    $searcher.Online = 1
}

$pending = $searcher.Search('IsInstalled=0 AND IsHidden=0')
foreach($entry in $pending.Updates) {
    Write-host Title: $entry.Title
    foreach($cat in $entry.Categories) {
        Write-host Category: $cat.Name
    }
}
