@echo off
echo Building Mnemosyne Photo Cloud Server...
echo.
go build -ldflags="-s -w" -o mnemosyne.exe .
if %errorlevel% equ 0 (
    echo.
    echo Build successful! Created mnemosyne.exe
    echo.
    echo To start the server, run:
    echo   mnemosyne.exe
    echo or double-click start.bat
) else (
    echo.
    echo Build failed! Please check for errors above.
)
pause

