@echo off

setlocal enabledelayedexpansion
set state=%1
set message=%~2

if "%state%" neq "0" if "%state%" neq "1" if "%state%" neq "2" if "%state%" neq "3" (
    echo Invalid state argument. Please provide one of: 0, 1, 2, 3
    exit /b 3
)

if not "!message!" == "" (
    set "message=: !message!"
)

if "%state%" equ "0" (
    echo OK!message!
    set exitStatus=0
) else if "%state%" equ "1" (
    echo WARNING!message!
    set exitStatus=1
) else if "%state%" equ "2" (
    echo CRITICAL!message!
    set exitStatus=2
) else if "%state%" equ "3" (
    echo UNKNOWN!message!
    set exitStatus=3
)

exit /b %exitStatus%
