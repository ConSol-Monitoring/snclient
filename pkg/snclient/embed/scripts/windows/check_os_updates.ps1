# check for windows updates
# usage: .\check_os_updates.ps1
#
# docs:
# https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nf-wuapi-iupdatesearcher-search
# https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iupdate
#
$Online = $false
if ($env:ONLINE_SEARCH -eq "1") {
    $Online = $true
}

# ensure output is utf8
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::UTF8

$update = new-object -com Microsoft.update.Session
if ($update -eq $null) { Write-Host "failed to get Microsoft.update.Session"; exit 1; }

$searcher = $update.CreateUpdateSearcher()
if ($searcher -eq $null) { Write-Host "failed to create update searcher"; exit 1; }

$searcher.Online = 0
if ($Online) {
    $searcher.Online = 1
}

function Get-UpdateProperty($Object, $Name) {
    $ErrorActionPreference = 'Stop'
    try {
        $value = $Object.$Name
        if ($null -ne $value) {
            if ($value -is [datetime]) {
                $value.ToUniversalTime().ToString('o')
            } elseif ($value -is [decimal]) {
                [long]$value
            } else {
                $value
            }
        } else {
            $null
        }
    } catch {
        # Older Windows Update interfaces and non-driver updates may lack properties.
        $null
    }
}

$pending = $searcher.Search('IsInstalled=0 AND IsHidden=0')
$updates = @(foreach($entry in $pending.Updates) {
    if ($entry.DeploymentAction -eq 4 -and $env:INCLUDE_OPTIONAL -ne "1") {
        continue
    }
    $identity = Get-UpdateProperty $entry 'Identity'
    [PSCustomObject]@{
        Title = $entry.Title
        Categories = @(foreach($cat in $entry.Categories) { $cat.Name })
        Type = [int]$entry.Type
        IsBeta = [bool]$entry.IsBeta
        IsMandatory = [bool]$entry.IsMandatory
        DeploymentAction = [int]$entry.DeploymentAction
        KBArticleIDs = @(Get-UpdateProperty $entry 'KBArticleIDs')
        MsrcSeverity = Get-UpdateProperty $entry 'MsrcSeverity'
        IsDownloaded = Get-UpdateProperty $entry 'IsDownloaded'
        RebootRequired = Get-UpdateProperty $entry 'RebootRequired'
        LastDeploymentChangeTime = Get-UpdateProperty $entry 'LastDeploymentChangeTime'
        UpdateID = Get-UpdateProperty $identity 'UpdateID'
        RevisionNumber = Get-UpdateProperty $identity 'RevisionNumber'
        BrowseOnly = Get-UpdateProperty $entry 'BrowseOnly'
        CveIDs = @(Get-UpdateProperty $entry 'CveIDs')
        MaxDownloadSize = Get-UpdateProperty $entry 'MaxDownloadSize'
        DriverProvider = Get-UpdateProperty $entry 'DriverProvider'
        DriverModel = Get-UpdateProperty $entry 'DriverModel'
        DriverVerDate = Get-UpdateProperty $entry 'DriverVerDate'
    }
})
ConvertTo-Json -InputObject $updates -Depth 3 -Compress
