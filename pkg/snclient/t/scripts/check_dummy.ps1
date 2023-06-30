param (
    [Parameter(Position = 0, Mandatory = $true)]
    [ValidateSet("0", "1", "2", "3")]
    [string]$state,
    [Parameter(Position = 1)]
    [string]$message = ""
)

if ($state -notin ("0", "1", "2", "3")) {
    Write-Host "Invalid state argument. Please provide one of: 0, 1, 2, 3"
    exit 3
}
if (![string]::IsNullOrEmpty($message)) {
    $message = ": $message"
}

switch ($state) {
    "0" {
        Write-Host "OK:$message"
        $exitStatus = 0
    }
    "1" {
        Write-Host "WARNING:$message"
        $exitStatus = 1
    }
    "2" {
        Write-Host "CRITICAL:$message"
        $exitStatus = 2
    }
    "3" {
        Write-Host "UNKNOWN:$message"
        $exitStatus = 3
    }
}

exit $exitStatus

