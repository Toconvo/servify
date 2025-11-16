# Servify Website

Static website (marketing/landing) for Servify. Built as plain HTML/CSS/JS for simple hosting behind any static server or CDN.

Structure:
- index.html            Landing page
- assets/css/style.css  Stylesheet
- assets/js/main.js     Small client-side interactions
- assets/img/           Images (logo, illustrations)

Local preview:
- Simple Python server:
  - python3 -m http.server -d apps/website 8081
  - Open http://localhost:8081

Production hosting:
- Any static hosting (Nginx/Apache/CDN/Pages)
- Set cache headers for /assets/**

