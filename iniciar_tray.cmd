@echo off
title CloudflareNetConnect
cd /d "%~dp0"
echo Iniciando CloudflareNetConnect en la bandeja del sistema...
bin\CloudflareNetConnect.exe -config config\netconnect.json
pause
