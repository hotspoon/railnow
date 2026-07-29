/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './templates/**/*.{templ,go}',
    './public/js/**/*.js',
  ],
  darkMode: 'class',
  theme: { extend: {} },
  plugins: [],
};
