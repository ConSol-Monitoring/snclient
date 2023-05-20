param ($out="snclient.msi", $arch="amd64", $major="0", $minor="0", $rev="1", $sha="aaaaa")

<#
If (-Not (Test-Path -Path ".\dotnetfx35setup.exe" )) {
  Invoke-WebRequest -UseBasicParsing `
    -Uri https://download.microsoft.com/download/0/6/1/061F001C-8752-4600-A198-53214C69B51F/dotnetfx35setup.exe `
    -OutFile dotnetfx35setup.exe
  & ".\dotnetfx35setup.exe" "/q"
}
#>

If (-Not (Test-Path -Path "C:\Program Files (x86)\WiX Toolset v3.11\bin\candle.exe" )) {
  If (-Not (Test-Path -Path ".\wix311.exe" )) {
    Invoke-WebRequest -UseBasicParsing `
      -Uri https://github.com/wixtoolset/wix3/releases/download/wix3112rtm/wix311.exe `
      -OutFile wix311.exe
  }
  & ".\wix311.exe" "/q"
}

$win_arch = "$arch"
if ("$arch" -eq "386")   { $win_arch = "x86" }
if ("$arch" -eq "amd64") { $win_arch = "x64" }

& 'C:\Program Files (x86)\WiX Toolset v3.11\bin\candle.exe' .\packaging\windows\snclient.wxs `
  -arch $win_arch `
  -dPlatform="$($win_arch)" `
  -dMajorVersion="$major" `
  -dMinorVersion="$minor" `
  -dRevisionNumber="$rev" `
  -dGitSha="$sha"
& "C:\Program Files (x86)\WiX Toolset v3.11\bin\light.exe" ".\snclient.wixobj"

Move-Item -Path ./snclient.msi -Destination "$out"
