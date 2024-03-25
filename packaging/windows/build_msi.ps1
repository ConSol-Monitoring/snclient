param ($out="snclient.msi", $arch="amd64", $major="0", $minor="0", $rev="1", $sha="unknown")

$ProgressPreference = 'SilentlyContinue'

If (-Not (Test-Path -Path "windist" )) {
  Write-Output "ERROR: windist folder is missing."
  Exit 1
}

If (-Not (Test-Path -Path "C:\Program Files (x86)\WiX Toolset v3.14\bin\candle.exe" )) {
  If (-Not (Test-Path -Path ".\wix314.exe" )) {
    Invoke-WebRequest -UseBasicParsing `
      -Uri https://github.com/wixtoolset/wix3/releases/download/wix3141rtm/wix314.exe `
      -OutFile wix314.exe
  }
  ls
  & ".\wix314.exe" "/q"
}

$win_arch = "$arch"
$go_arch  = "$arch"
if ("$arch" -eq "386")    { $win_arch = "x86" }
if ("$arch" -eq "i386")   { $win_arch = "x86"; $go_arch = "386" }
if ("$arch" -eq "amd64")  { $win_arch = "x64" }
if ("$arch" -eq "x86_64") { $win_arch = "x64"; $go_arch = "amd64" }

Copy-Item .\windist\windows_exporter-$go_arch.exe .\windist\windows_exporter.exe

& 'C:\Program Files (x86)\WiX Toolset v3.14\bin\candle.exe' .\packaging\windows\snclient.wxs `
  -arch $win_arch `
  -dPlatform="$win_arch" `
  -dMajorVersion="$major" `
  -dMinorVersion="$minor" `
  -dRevisionNumber="$rev" `
  -dGitSha="$sha"
If (-Not $?) {
  Exit 1
}

& "C:\Program Files (x86)\WiX Toolset v3.14\bin\light.exe" ".\snclient.wixobj" -ext WixUtilExtension.dll -ext WixUIExtension.dll
If (-Not $?) {
  Exit 1
}


if ("$out" -ne "snclient.msi") {
  If (Test-Path -Path "$out" ) {
    Remove-Item $out
  }
  Move-Item -Path .\snclient.msi -Destination $out
}
Write-Output "build $out successfully."