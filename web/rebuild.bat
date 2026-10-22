@echo off
rmdir /s /q ".next" 2>nul
rmdir /s /q "node_modules/.cache" 2>nul
npm run build
npm run start
