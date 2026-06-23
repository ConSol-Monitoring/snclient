# list scheduled tasks in json format 
# this version uses the Schedule.Service COM API
# it avoids importing the ScheduledTasks module, which can be extremely slow
# on machines with EDR/antivirus solutions that scan modules via AMSI
# usage: .\scheduled_tasks.ps1 [-title <pattern>] [-folder <path>] [-recursive <true|false>]

# Parse named arguments (for standalone invocation).
# When called via snclient, parameters are defined at the top of the script
# the parameters will be parsed without looking at $args
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
        if ($args[$i] -eq '-hidden' -and $i + 1 -lt $args.Count) {
            $hidden = $args[$i + 1]
            $i++
            continue
        }
    }
}

# Apply defaults when variables are not defined (neither by snclient parameter injection nor by args)
if (!$title) { $title = '*' }
if (!$folder) { $folder = '\' }
if (!$recursive) { $recursive = 'true' }
if (!$hidden) { $hidden = 'true' }

# debug the parameters/arguments
[Console]::Error.WriteLine(('title: ' + $title ))
[Console]::Error.WriteLine(('folder: ' + $folder ))
[Console]::Error.WriteLine(('recursive: ' + $recursive ))
[Console]::Error.WriteLine(('hidden: ' + $hidden ))

# ensure output is utf8
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::UTF8

# Print powershell version
[Console]::Error.WriteLine(('Powershell version table: ' + ($PSVersionTable | ConvertTo-Json -Compress)))

$sw = [System.Diagnostics.Stopwatch]::StartNew()
$scheduler = New-Object -ComObject Schedule.Service
$scheduler.Connect()
$sw.Stop()
[Console]::Error.WriteLine(('COM Schedule.Service connect took {0:F2} ms' -f $sw.Elapsed.TotalMilliseconds))

$sw = [System.Diagnostics.Stopwatch]::StartNew()
$tasks = [System.Collections.Generic.List[object]]::new()
try {
    $targetFolder = $scheduler.GetFolder($folder)
    $folderQueue = [System.Collections.Queue]::new()
    $folderQueue.Enqueue($targetFolder)
    while ($folderQueue.Count -gt 0) {
        $currentFolder = $folderQueue.Dequeue()
        # TASK_ENUM_HIDDEN = 1, include hidden tasks
        # Call GetTasks() using TASK_ENUM_HIDDEN
        if ($hidden -eq 'true'){
            $getTasksArg = 1
        } else {
            $getTasksArg = 0
        }
        foreach ($t in $currentFolder.GetTasks($getTasksArg)) {
            $tasks.Add($t)
        }
        if ($recursive -eq 'true') {
            foreach ($sub in $currentFolder.GetFolders(0)) {
                $folderQueue.Enqueue($sub)
            }
        }
    }
} catch {
    $tasks = [System.Collections.Generic.List[object]]::new()
}
$sw.Stop()
[Console]::Error.WriteLine(('Task enumeration took {0:F2} ms' -f $sw.Elapsed.TotalMilliseconds))

if ($title -ne '*') {
    $filtered = [System.Collections.Generic.List[object]]::new()
    foreach ($t in $tasks) {
        if ($t.Name -eq $title) {
            $filtered.Add($t)
        }
    }
    $tasks = $filtered
}

$sw = [System.Diagnostics.Stopwatch]::StartNew()
$results = [System.Collections.Generic.List[object]]::new()
foreach ($task in $tasks) {
    $def = $task.Definition
    $taskPath = $task.Path.Substring(0, $task.Path.Length - $task.Name.Length)

    $actions = [System.Collections.Generic.List[object]]::new()
    foreach ($action in $def.Actions) {
        # COM IAction.Type: 0 = TASK_ACTION_EXEC (the only type with Path/Arguments/WorkingDirectory)
        if ($action.Type -eq 0) {
            $actions.Add(
                [PSCustomObject]@{
                    Arguments        = [string]$action.Arguments
                    Execute          = [string]$action.Path
                    Id               = [string]$action.Id
                    PSComputerName   = ''
                    WorkingDirectory = [string]$action.WorkingDirectory
                }
            )
        }
    }

    $results.Add(
        [PSCustomObject]@{
            TaskName           = $task.Name
            TaskPath           = $taskPath
            State              = [int]$task.State
            Description        = [string]$def.RegistrationInfo.Description
            PSComputerName     = ''
            URI                = $task.Path
            Version            = [string]$def.RegistrationInfo.Version
            LastRunTime        = $task.LastRunTime
            LastTaskResult     = [BitConverter]::ToUInt32([BitConverter]::GetBytes([int32]$task.LastTaskResult), 0)
            NextRunTime        = $task.NextRunTime
            NumberOfMissedRuns = [int64]$task.NumberOfMissedRuns
            UserId             = [string]$def.Principal.UserId
            Enabled            = [bool]$task.Enabled
            Priority           = [int64]$def.Settings.Priority
            Hidden             = [bool]$def.Settings.Hidden
            ExecutionTimeLimit = [string]$def.Settings.ExecutionTimeLimit
            Actions            = @($actions)
        }
    )
}
$sw.Stop()
[Console]::Error.WriteLine(('Populating results list took {0:F2} ms' -f $sw.Elapsed.TotalMilliseconds))
[Console]::Error.WriteLine(('Results list has {0} elements' -f $results.Count))

$sw = [System.Diagnostics.Stopwatch]::StartNew()
if ($results.Count -gt 0) {
    ConvertTo-Json -InputObject $results -Depth 4
} else {
    '[]'
}
$sw.Stop()
[Console]::Error.WriteLine(('Converting to JSON took {0:F2} ms' -f $sw.Elapsed.TotalMilliseconds))

exit 0