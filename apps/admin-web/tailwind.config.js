/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    screens: {
      xs: '360px', sm: '430px', md: '768px', lg: '1024px', xl: '1280px', '2xl': '1440px',
    },
    extend: {
      colors: {
        ink: '#111111',
        stone: {
          50: '#fafafa', 100: '#f5f5f4', 200: '#e7e5e4', 300: '#d6d3d1',
          400: '#a8a29e', 500: '#78716c', 600: '#57534e', 700: '#44403c',
          800: '#292524', 900: '#1c1917',
        },
        accent: '#8b5e3c',
      },
      fontFamily: { sans: ['Inter', 'system-ui', 'sans-serif'] },
      boxShadow: {
        sm: '0 1px 3px 0 rgb(0 0 0 / 0.06)',
        md: '0 4px 12px 0 rgb(0 0 0 / 0.08)',
      },
    },
  },
  plugins: [],
}
