# command_line_test.ps1
# A simple PowerShell script for testing purposes

param(
    [Parameter(Mandatory = $false)]
    [object]$option1,

    [Parameter(Mandatory = $false)]
    [object]$option2,

    [Parameter(Mandatory = $false)]
    [object]$option3,

    [Parameter(Mandatory = $false)]
    [object]$option4,

    [Parameter(Mandatory = $false)]
    [object]$option5,

    [Parameter(Mandatory = $false, Position = 0, ValueFromRemainingArguments = $true)]
    [object]$Arguments
)

Write-Host "Raw Commandline: $($MyInvocation.Line)"
Write-Host ""

Write-Host "PowerShell Version: $($PSVersionTable.PSVersion.ToString())"
Write-Host ""

Write-Host "PowerShell Version Details:" -ForegroundColor Yellow
$PSVersionTable | Format-List
Write-Host ""

Write-Host "Process Information:" -ForegroundColor Yellow
$process = Get-Process -Id $PID
Write-Host "Process ID: $($process.Id)"
Write-Host "Process Name: $($process.ProcessName)"
Write-Host "Process Commandline: $($process.CommandLine)"
Write-Host ""

Write-Host "Script Information:" -ForegroundColor Yellow
Write-Host "Script Name: $MyInvocation.MyCommand.Name"
Write-Host "Script Path: $MyInvocation.MyCommand.Path"
Write-Host ""

Write-Host "Working Directory: $($PWD.Path)"
Write-Host ""

Write-Host "Environment Info:" -ForegroundColor Yellow
Write-Host "OS: $($env:OS)"
Write-Host "Computer Name: $($env:COMPUTERNAME)"
Write-Host "User: $($env:USERNAME)"
Write-Host ""

Write-Host "Script Execution Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff')"
Write-Host ""

Write-Host "Arguments Received:" -ForegroundColor Yellow
if ($Arguments) {
    for ($i = 0; $i -lt $Arguments.Length; $i++) {
        Write-Host "Argument | [$i] : $($Arguments[$i])"
    }
}
Write-Host ""

Write-Host "Bound Parameters:" -ForegroundColor Yellow
$MyInvocation.BoundParameters.GetEnumerator() | ForEach-Object {
    $paramName = $_.Key
    $paramValue = $_.Value
    
    # Safely get the type (checking for $null first to prevent errors)
    $typeDisplay = "null"
    if ($null -ne $paramValue) {
        $typeDisplay = $paramValue.GetType().Name
    }

    Write-Host "Bound Parameter | Name: $paramName | Type: $typeDisplay | Value: $paramValue"
}
Write-Host ""