Option Explicit

Dim state, message, exitStatus
state = WScript.Arguments.Item(0)
If WScript.Arguments.Count > 1 Then
    message = WScript.Arguments.Item(1)
Else
    message = ""
End If

If state <> "0" And state <> "1" And state <> "2" And state <> "3" Then
    WScript.Echo "Invalid state argument. Please provide one of: 0, 1, 2, 3"
    exitStatus = UNKNOWN
Else
    If message <> "" Then
        message = ": " & message
    End If

    Select Case state
        Case "0"
            WScript.Echo "OK" & message
            exitStatus = 0
        Case "1"
            WScript.Echo "WARNING" & message
            exitStatus = 1
        Case "2"
            WScript.Echo "CRITICAL" & message
            exitStatus = 2
        Case "3"
            WScript.Echo "UNKNOWN" & message
            exitStatus = 3
    End Select
End If

WScript.Quit(exitStatus)

