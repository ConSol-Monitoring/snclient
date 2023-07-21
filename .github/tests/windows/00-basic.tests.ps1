$global:msiFilePath = "snclient.msi"
$global:snclient    = "C:\Program Files\snclient\snclient.exe"

Describe "MSI Installation Test" {
    Context "Install snclient.msi" {
        It "Should install the .msi successfully" {
            Start-Process msiexec -ArgumentList "/i $msiFilePath /qn" -Wait
        }
    }

    It "Wait until the snclient.exe appears" {
        $maxWaitTimeSeconds = 30
        $elapsedTime = 0

        while (-Not (Test-Path $snclient)) {
            if ($elapsedTime -ge $maxWaitTimeSeconds) {
                throw "$snclient did not appear within $maxWaitTimeSeconds seconds."
            }

            Start-Sleep -Seconds 1
            $elapsedTime += 1
        }
    }

    Context "check_time" {
        It "Should run the application after installation" {
            $res = Invoke-Expression -Command "&'$snclient' run check_uptime crit=uptime<2s warn=uptime<1s"
            $res | Should -Match "OK: uptime"
            $LASTEXITCODE | Should -Be 0
        }
    }

    Context "Uninstall snclient.msi" {
        It "Should uninstall the .msi successfully" {
            Start-Process msiexec -ArgumentList "/x $msiFilePath /qn" -Wait
            $LASTEXITCODE | Should -Be 0
        }
    }

    It "Wait until the snclient.exe disappears" {
        $maxWaitTimeSeconds = 30
        $elapsedTime = 0

        while (Test-Path $snclient) {
            if ($elapsedTime -ge $maxWaitTimeSeconds) {
                throw "$snclient did not disappear within $maxWaitTimeSeconds seconds."
            }

            Start-Sleep -Seconds 1
            $elapsedTime += 1
        }
    }
}
