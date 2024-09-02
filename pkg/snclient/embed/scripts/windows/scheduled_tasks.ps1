# list scheduled tasks in json format
# usage: .\scheduled_tasks.ps1
#

# ensure output is utf8
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::UTF8

Get-ScheduledTask | ForEach-Object {
    $task = $_
    $taskInfo = Get-ScheduledTaskInfo -TaskName $task.TaskName -TaskPath $task.TaskPath

    # Extract command line and arguments from the actions
    $actions = @($task.Actions | ForEach-Object {
        [PSCustomObject]@{
            Execute  = $_.Execute
            Arguments = $_.Arguments
        }
    })

    # Combine task properties with task info properties
    [PSCustomObject]@{
        TaskName        = $task.TaskName
        TaskPath        = $task.TaskPath
        State           = $task.State
        Description     = $task.Description
        LastRunTime     = $taskInfo.LastRunTime
        NextRunTime     = $taskInfo.NextRunTime
        LastTaskResult  = $taskInfo.LastTaskResult
        MissedRuns      = $taskInfo.NumberOfMissedRuns
        Author          = $task.Principal.UserId
        Enabled         = $task.Settings.Enabled
        Priority        = $task.Settings.Priority
        Hidden          = $task.Settings.Hidden
        TimeLimit       = $task.Settings.ExecutionTimeLimit
        Actions         = $actions
    }
} | ConvertTo-Json -Depth 4
