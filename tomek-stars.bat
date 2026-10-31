@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Play-Windows.ps1" -Background
if errorlevel 1 pause
