@echo off
setlocal
REM Autostart WSL + Node2Mint + Cloudflare Tunnel saat Windows logon.
REM WSL tidak hidup sendiri setelah reboot; launcher ini menyalakan origin dulu,
REM menunggu Caddy menjawab, lalu menjalankan cloudflared via HTTP/2.
set "LOG=%LOCALAPPDATA%\node2mint-autostart.log"
>>"%LOG%" echo.
>>"%LOG%" echo ==== %DATE% %TIME% Node2Mint autostart ====
wsl.exe -d Ubuntu -u root -- bash -lc "bash /root/node2mint/deploy/wsl-autostart.sh" >> "%LOG%" 2>&1
exit /b %ERRORLEVEL%
