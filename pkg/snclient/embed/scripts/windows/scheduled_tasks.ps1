# list scheduled tasks in json format
# usage: .\scheduled_tasks.ps1 [-title <pattern>] [-folder <path>] [-recursive <true|false>]

# Parse named arguments (for standalone invocation).
# When called via snclient, variables are injected at the top of the script instead,
# so $args will be empty and this loop does nothing.
if ($args) {
    for ($i = 0; $i -lt $args.Count; $i++) {
        if ($args[$i] -eq '-title' -and $i + 1 -lt $args.Count) {
            $title = $args[$i + 1]
            $i++
            continue
        }
        if ($args[$i] -eq '-folder' -and $i + 1 -lt $args.Count) {
            $folder = $args[$i + 1]
            $i++
            continue
        }
        if ($args[$i] -eq '-recursive' -and $i + 1 -lt $args.Count) {
            $recursive = $args[$i + 1]
            $i++
            continue
        }
    }
}

# Apply defaults when variables are not defined (neither by snclient injection nor by args)
if (!$title) { $title = '*' }
if (!$folder) { $folder = '\' }
if (!$recursive) { $recursive = 'false' }

# ensure output is utf8
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::UTF8

$params = @{}
if ($title -ne '*') {
    $params.TaskName = $title
}
if ($recursive -eq 'true') {
    $params.TaskPath = $folder + '*'
} else {
    $params.TaskPath = $folder
}

try {
    $tasks = Get-ScheduledTask @params -ErrorAction Stop
} catch {
    $tasks = @()
}

$results = @()
foreach ($task in $tasks) {
    $taskInfo = Get-ScheduledTaskInfo -TaskName $task.TaskName -TaskPath $task.TaskPath

    # Extract command line and arguments from the actions
    # Get-ScheduledTask -TaskName "XYZ" | Select-Object -ExpandProperty Actions | Get-Member -MemberType Property
    $actions = @($task.Actions | ForEach-Object {
        [PSCustomObject]@{
            Arguments = $_.Arguments
            Execute  = $_.Execute
            Id = $_.Id
            PSComputerName = $_.PSComputerName
            WorkingDirectory = $_.WorkingDirectory
        }
    })

    # Get-ScheduledTask -TaskName "XYZ" | Select-Object -ExpandProperty Triggers | Get-Member -MemberType Property
    $triggers = @($task.Triggers | ForEach-Object {
        [PSCustomObject]@{
            DaysInterval = $_.DaysInterval
            Enabled = $_.Enabled
            EndBoundary = $_.EndBoundary
            ExecutionTimeLimit = $_.ExecutionTimeLimit
            Id = $_.Id
            RandomDelay = $_.RandomDelay
            Repetition = $_.Repetition
            StartBoundary = $_.StartBoundary
        }
    })

    # Get-ScheduledTask -TaskName "XYZ" | Select-Object -ExpandProperty Settings | Get-Member -MemberType Property
    $settings = [PSCustomObject]@{
            AllowDemandStart = $task.Settings.AllowDemandStart
            AllowHardTerminate = $task.Settings.AllowHardTerminate
            DeleteExpiredTaskAfter = $task.Settings.DeleteExpiredTaskAfter
            DisallowStartIfOnBatteries = $task.Settings.DisallowStartIfOnBatteries
            DisallowStartOnRemoteAppSession = $task.Settings.DisallowStartOnRemoteAppSession
            Enabled = $task.Settings.Enabled
            ExecutionTimeLimit = $task.Settings.ExecutionTimeLimit
            Hidden = $task.Settings.Hidden
            IdleSettings = $task.Settings.IdleSettings
            MaintenanceSettings = $task.Settings.MaintenanceSettings
            NetworkSettings = $task.Settings.NetworkSettings
            Priority = $task.Settings.Priority
            PSComputerName = $task.Settings.PSComputerName
            RestartCount = $task.Settings.RestartCount
            RestartInterval = $task.Settings.RestartInterval
            RunOnlyIfIdle = $task.Settings.RunOnlyIfIdle
            RunOnlyIfNetworkAvailable = $task.Settings.RunOnlyIfNetworkAvailable
            StartWhenAvailable = $task.Settings.StartWhenAvailable
            StopIfGoingOnBatteries = $task.Settings.StopIfGoingOnBatteries
            UseUnifiedSchedulingEngine = $task.Settings.UseUnifiedSchedulingEngine
            Volatile = $task.Settings.Volatile
            WakeToRun = $task.Settings.WakeToRun
        }

    # Get-ScheduledTask -TaskName "XYZ" | Select-Object -ExpandProperty Principal | Get-Member -MemberType Property
    $principal = [PSCustomObject]@{
            DisplayName = $task.Principal.DisplayName
            Id = $task.Principal.Id
            GroupId = $task.Principal.GroupId
            PSComputerName = $task.Principal.PSComputerName
            RequiredPrivilege = $task.Principal.RequiredPrivilege
            UserId = $task.Principal.UserId
        }

    # Combine task properties with task info properties
    # Get-ScheduledTask -TaskName "XYZ" | Get-Member -MemberType Property
    # Get-ScheduledTaskInfo -TaskName "XYZ" | Get-Member -MemberType Property
    $results += [PSCustomObject]@{
        TaskName                = $task.TaskName
        TaskPath                = $task.TaskPath
        State                   = $task.State
        Description             = $task.Description
        PSComputerName          = $task.PSComputerName
        URI                     = $task.URI
        Version                 = $task.Version
        LastRunTime             = $taskInfo.LastRunTime
        LastTaskResult          = $taskInfo.LastTaskResult
        NextRunTime             = $taskInfo.NextRunTime
        NumberOfMissedRuns      = $taskInfo.NumberOfMissedRuns
        UserId                  = $principal.UserId
        Enabled                 = $settings.Enabled
        Priority                = $settings.Priority
        Hidden                  = $settings.Hidden
        ExecutionTimeLimit      = $settings.ExecutionTimeLimit
        Principal               = $principal
        Actions                 = $actions
        Triggers                = $triggers
        Settings                = $settings
    }
}

if ($results.Count -gt 0) {
    ConvertTo-Json -InputObject $results -Depth 4
} else {
    '[]'
}
