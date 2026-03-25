@echo off
setlocal

echo Building frontend...
cd frontend
call npm install
call npx next build
cd ..

echo Copying static files...
if exist server\internal\frontend\dist rmdir /s /q server\internal\frontend\dist
xcopy /s /e /i /q frontend\out server\internal\frontend\dist

echo Building binary...
cd server
go build -o ..\bin\airplay-server.exe .\cmd\
cd ..

echo.
echo Starting AirPlay Server...
bin\airplay-server.exe %*
