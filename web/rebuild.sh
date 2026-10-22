#!/bin/bash
cd "/c/Users/YM/Desktop/PROJECT SERIUS/WEB - PROJECT AI SELL/ai-sell-modern/web"
npx rimraf .next 2>/dev/null || true
npx rimraf node_modules/.cache 2>/dev/null || true
npm run build
npm run start
