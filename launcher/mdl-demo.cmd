@echo off
setlocal EnableExtensions
rem mdl-demo.cmd - run Moodle/MuTMS demo containers on Windows 11 (WSL containers, wslc).
rem Project, docs and the latest version of this script:
rem   https://github.com/mutms/mdl-demo   (see WINDOWS.md)
rem
rem   mdl-demo create [NNNN] [--name="Fancy demo"] [--image=REF] [--open]
rem   mdl-demo start|stop|delete [NNNN]
rem   mdl-demo list
rem   mdl-demo gc
rem   mdl-demo install^|uninstall   (print setup/removal steps; change nothing)
rem
rem NNNN is the demo's number: the port of its management console. The
rem container is named mdl-demo-NNNN, the console is http://127.0.0.1:NNNN and
rem the Moodle site is on the next port, NNNN+1. Without a number, 8081 (and
rem 8082 for the site). Pick another number to run several demos side by side.
rem
rem Inside the container the ports are fixed (8081 console, 8082 site); this
rem script maps NNNN and NNNN+1 onto them and passes the number in as
rem MDL_DEMO_PORT, so the console knows its own address.

set "IMAGE=ghcr.io/mutms/mdl-demo:latest"
if defined MDL_DEMO_IMAGE set "IMAGE=%MDL_DEMO_IMAGE%"
set "PORT=8081"
set "NAME="
set "OPEN="
set "ALL="
set "POSITIONAL=0"

set "CMD=%~1"
if "%CMD%"=="" set "CMD=help"
if /i "%CMD%"=="help" goto usage
if /i "%CMD%"=="--help" goto usage
if /i "%CMD%"=="-h" goto usage
rem install/uninstall only PRINT the steps; they change nothing and must work
rem before wslc exists, so handle them before the runtime check below.
if /i "%CMD%"=="install" goto install
if /i "%CMD%"=="uninstall" goto uninstall
shift

rem Two checks, two different fixes: wslc is not installed at all, or it is
rem installed but WSL is not ready to run containers. Help text goes to the
rem user before any command is attempted.
where wslc >nul 2>&1
if errorlevel 1 goto nowslc
wslc ps >nul 2>&1
if errorlevel 1 goto wslcdown

rem cmd.exe treats "=" as an argument separator, so --name="Fancy demo"
rem arrives as two arguments: --name and "Fancy demo" (quotes stripped by %~2).
:parse
if "%~1"=="" goto parsed
set "ARG=%~1"
if /i "%ARG%"=="--name"  ( set "NAME=%~2" & shift & shift & goto parse )
if /i "%ARG%"=="--image" ( set "IMAGE=%~2" & shift & shift & goto parse )
if /i "%ARG%"=="--open" ( set "OPEN=1" & shift & goto parse )
if /i "%ARG%"=="--all"  ( set "ALL=1" & shift & goto parse )
if /i "%ARG%"=="--help" goto usage
if /i "%ARG%"=="-h"     goto usage
echo %ARG%| findstr /r "^[0-9][0-9]*$" >nul
if errorlevel 1 (
    echo mdl-demo: unexpected argument "%ARG%" - quote names with spaces: --name="My demo" 1>&2
    exit /b 1
)
set /a POSITIONAL+=1
if %POSITIONAL% GTR 1 (
    echo mdl-demo: only one demo number allowed 1>&2
    exit /b 1
)
set "PORT=%ARG%"
shift
goto parse
:parsed

if %PORT% LSS 1024 goto badport
if %PORT% GTR 65534 goto badport
set "CNAME=mdl-demo-%PORT%"
set /a SITE=PORT+1

if /i "%CMD%"=="create" goto create
if /i "%CMD%"=="start"  goto start
if /i "%CMD%"=="stop"   goto stop
if /i "%CMD%"=="delete" goto delete
if /i "%CMD%"=="list"   goto list
if /i "%CMD%"=="gc"     goto gc
echo mdl-demo: unknown command "%CMD%" (see: mdl-demo help) 1>&2
exit /b 1

:create
wslc inspect %CNAME% >nul 2>&1
if not errorlevel 1 (
    echo mdl-demo: %CNAME% already exists - "mdl-demo start %PORT%" starts it, "mdl-demo delete %PORT%" removes it 1>&2
    exit /b 1
)
set "ENVS=-e MDL_DEMO_PORT=%PORT%"
if defined NAME set "ENVS=%ENVS% -e "MDL_DEMO_NAME=%NAME%""
wslc run -d --name %CNAME% %ENVS% -p 127.0.0.1:%PORT%:8081 -p 127.0.0.1:%SITE%:8082 %IMAGE% >nul
if errorlevel 1 exit /b 1
if defined NAME (echo created %CNAME% ^(%NAME%^)) else (echo created %CNAME%)
echo set up your demo site in the console: http://127.0.0.1:%PORT%
if defined OPEN call :openconsole
exit /b 0

:start
wslc inspect %CNAME% >nul 2>&1
if errorlevel 1 (
    echo mdl-demo: %CNAME% does not exist - "mdl-demo create %PORT%" makes it 1>&2
    exit /b 1
)
wslc start %CNAME% >nul
if errorlevel 1 exit /b 1
echo started %CNAME% - console: http://127.0.0.1:%PORT%
if defined OPEN call :openconsole
exit /b 0

:stop
wslc inspect %CNAME% >nul 2>&1
if errorlevel 1 (
    echo mdl-demo: %CNAME% does not exist 1>&2
    exit /b 1
)
wslc stop %CNAME% >nul
if errorlevel 1 exit /b 1
echo stopped %CNAME% (start it again with "mdl-demo start %PORT%")
exit /b 0

:delete
wslc inspect %CNAME% >nul 2>&1
if errorlevel 1 (
    echo mdl-demo: %CNAME% does not exist 1>&2
    exit /b 1
)
wslc stop %CNAME% >nul 2>&1
wslc rm %CNAME% >nul
if errorlevel 1 exit /b 1
echo deleted %CNAME% and its demo site
exit /b 0

:list
wslc ps -a | findstr /c:"CONTAINER" /c:"mdl-demo-"
exit /b 0

:gc
if defined ALL (
    echo removing all unused images ^(they re-download when a demo needs them^)...
    wslc image prune --all
) else (
    echo removing unused ^(dangling^) images...
    wslc image prune
    echo still low on disk? "mdl-demo gc --all" removes unused demo images too.
)
exit /b 0

:install
echo Setting up mdl-demo on Windows 11 - just the steps, nothing is changed.
echo.
echo   1. Install the WSL containers preview:
echo          wsl --update --pre-release
echo      then open a NEW terminal window. Details:
echo          https://devblogs.microsoft.com/commandline/wsl-container-is-now-available-for-public-preview/
echo.
echo   2. Keep mdl-demo.cmd in a folder on your PATH, or run it from the
echo      folder you downloaded it into.
echo.
echo   3. Start your first demo:
echo          mdl-demo create
exit /b 0

:uninstall
echo Removing mdl-demo on Windows 11 - just the steps, nothing is changed.
echo.
echo   1. Delete your demos - this removes their sites and data:
echo          mdl-demo list
echo          mdl-demo delete NNNN     for each demo the list shows
echo.
echo   2. Reclaim the disk the images used:
echo          mdl-demo gc
echo          wslc image remove ghcr.io/mutms/mdl-demo
echo.
echo   3. Delete mdl-demo.cmd.
echo.
echo   4. Optional: remove the WSL containers preview if you use it for
echo      nothing else.
exit /b 0

rem Waits for the console to answer, then hands it to the default browser.
rem The container is running before the console is listening: "wslc run"
rem returns once the init has been started and the init opens the port a
rem moment later, so a bare "start" can win the race and land on a connection
rem error. ping is the wait because "timeout" refuses to run whenever the
rem script's input is redirected.
:openconsole
set "URL=http://127.0.0.1:%PORT%"
echo waiting for the console...
for /l %%i in (1,1,30) do (
    curl.exe -fs -o NUL "%URL%" >nul 2>&1
    if not errorlevel 1 (
        start "" "%URL%"
        goto :eof
    )
    ping -n 2 127.0.0.1 >nul
)
echo mdl-demo: the console has not answered yet - open %URL% when it does 1>&2
goto :eof

:nowslc
echo mdl-demo: WSL containers ^(wslc^) are not installed - run "mdl-demo install" for the setup steps. 1>&2
exit /b 1

:wslcdown
echo mdl-demo: wslc is installed but not responding. 1>&2
echo. 1>&2
echo   Make sure WSL is up to date and running: 1>&2
echo       wsl --update --pre-release 1>&2
echo       wsl --status 1>&2
echo   then try this command again. 1>&2
exit /b 1

:badport
echo mdl-demo: the demo number must be a port between 1024 and 65534 (got "%PORT%") 1>&2
exit /b 1

:usage
echo mdl-demo - run throwaway Moodle/MuTMS demo sites on Windows, in a container.
echo.
echo Usage: mdl-demo ^<command^> [NNNN] [options]
echo.
echo Commands:
echo   create [NNNN]   create and start a new demo (console on port NNNN, default 8081)
echo   start  [NNNN]   start a stopped demo
echo   stop   [NNNN]   stop a running demo (the site and its data are kept)
echo   delete [NNNN]   stop and remove a demo, including its site and data
echo   list            show all demos
echo   gc [--all]      reclaim disk from unused images (never touches containers);
echo                   --all also removes images no demo is using (e.g. the demo image)
echo   install         print how to set up mdl-demo on this system (changes nothing)
echo   uninstall       print how to remove mdl-demo from this system (changes nothing)
echo.
echo Options for create:
echo   --name="..."    label shown in the console heading, also the Moodle site name
echo   --image=REF     use a specific image, e.g. a custom offline build
echo                   (default: ghcr.io/mutms/mdl-demo:latest, or %%MDL_DEMO_IMAGE%%)
echo.
echo Options for create and start:
echo   --open          open the console in your browser once it answers
echo.
echo The demo's number NNNN is the port of its console: http://127.0.0.1:NNNN.
echo The Moodle site is on the next port, NNNN+1.
echo.
echo What this is, the full guide, and the latest version of this script:
echo   https://github.com/mutms/mdl-demo   (see WINDOWS.md)
exit /b 0
