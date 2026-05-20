/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Accent — teal (primary brand). Used sparingly — Linear-style restraint.
        accent: {
          50: '#f0fdfa',
          100: '#ccfbf1',
          200: '#99f6e4',
          300: '#5eead4',
          400: '#2dd4bf',
          500: '#14b8a6',
          600: '#0d9488',
          700: '#0f766e',
          800: '#115e59',
          900: '#134e4a',
          950: '#042f2e'
        },
        // Primary — indigo (semantic only: secondary protocol/info cues).
        primary: {
          50: '#eef2ff',
          100: '#e0e7ff',
          200: '#c7d2fe',
          300: '#a5b4fc',
          400: '#818cf8',
          500: '#6366f1',
          600: '#4f46e5',
          700: '#4338ca',
          800: '#3730a3',
          900: '#312e81',
          950: '#1e1b4b'
        },
        // Ink — Linear/Vercel-style cool near-black for ink buttons and headings.
        // Cooler & deeper than surface so primary buttons read as distinct.
        ink: {
          50: '#f7f8f9',
          100: '#eaecef',
          200: '#d6dadf',
          300: '#a9b1bc',
          400: '#76808d',
          500: '#525c69',
          600: '#373f4a',
          700: '#252a32',
          800: '#15181d',
          900: '#0c0e12',
          950: '#06080a'
        },
        // Surface — warm neutral. Tinted slightly toward stone/sand so the
        // page doesn't feel "Excel gray". Values approximate OKLCH(L 0.04C 60h).
        surface: {
          0: '#ffffff',
          50: '#fafaf9',
          100: '#f5f5f4',
          200: '#e7e5e4',
          300: '#d6d3d1',
          400: '#a8a29e',
          500: '#78716c',
          600: '#57534e',
          700: '#44403c',
          800: '#292524',
          900: '#1c1917',
          950: '#0c0a09'
        }
      },
      fontFamily: {
        // Geist Sans for UI, DM Sans fallback. Both loaded via Google Fonts
        // in index.html. system-ui sits at the tail just in case.
        sans: [
          'Geist',
          'DM Sans',
          'Inter',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        mono: [
          'Geist Mono',
          'JetBrains Mono',
          'ui-monospace',
          'SFMono-Regular',
          'Menlo',
          'Monaco',
          'Consolas',
          'monospace'
        ]
      },
      // Brand easing — Vercel/Linear-style "spring-out" feel without the cheap
      // elastic overshoot. Used everywhere we transition.
      transitionTimingFunction: {
        brand: 'cubic-bezier(0.16, 1, 0.3, 1)'
      },
      boxShadow: {
        // Default state — hairline only. Cards live on hairline, not shadow.
        // Premium UI principle: depth comes from contrast, not stacked drop shadows.
        hairline: '0 0 0 1px rgb(231 229 228 / 1)',
        // Subtle resting shadow (Marzban-style). Used only when card sits ON
        // a colored background, not on top of another card.
        card: '0 1px 2px 0 rgb(15 23 42 / 0.04)',
        // Hover lift — restrained, no big spread.
        'card-hover': '0 2px 8px -2px rgb(15 23 42 / 0.06), 0 1px 3px 0 rgb(15 23 42 / 0.05)',
        // Soft "lifted" surface for modals
        elevated: '0 12px 32px -4px rgb(15 23 42 / 0.18), 0 2px 8px rgb(0 0 0 / 0.04)',
        // Inset ring used on the active sidebar item — Xboard-style left accent bar.
        rail: 'inset 2px 0 0 0 rgb(13 148 136 / 1)',
        // Focus ring
        focus: '0 0 0 3px rgb(20 184 166 / 0.25)'
      },
      borderRadius: {
        // Premium UI corner radius scale — bumps default rounding so cards read
        // as soft pebbles, not hard tiles. Aligned with Marzban/Xboard/Linear.
        DEFAULT: '0.5rem', // 8px
        xl: '0.875rem',    // 14px (Tailwind default 12px)
        '2xl': '1.125rem', // 18px (Tailwind default 16px)
        '3xl': '1.5rem'    // 24px (Tailwind default)
      },
      // Semantic font scale — replaces magic numbers like text-[11px] / text-[28px].
      // Bumped one notch from Tailwind defaults so dense data screens read more
      // comfortably (per user feedback "字体有点小"). Modular ~1.2 ratio.
      fontSize: {
        // Override Tailwind defaults so all `text-sm` / `text-xs` consumers bump up.
        xs:            ['0.8125rem', { lineHeight: '1.125rem' }],   // 13px (was 12)
        sm:            ['0.9375rem', { lineHeight: '1.375rem' }],   // 15px (was 14)
        base:          ['1rem',      { lineHeight: '1.5rem' }],     // 16px (Tailwind default)
        lg:            ['1.125rem',  { lineHeight: '1.625rem' }],   // 18px
        xl:            ['1.25rem',   { lineHeight: '1.75rem' }],    // 20px
        '2xl':         ['1.5rem',    { lineHeight: '2rem', letterSpacing: '-0.01em' }], // 24px
        '3xl':         ['1.875rem',  { lineHeight: '2.25rem', letterSpacing: '-0.015em' }], // 30px

        // Custom semantic tokens.
        '2xs':         ['0.75rem',   { lineHeight: '1rem' }],       // 12px — eyebrow / dense meta
        eyebrow:       ['0.6875rem', { lineHeight: '1rem',  letterSpacing: '0.14em' }], // 11px uppercase
        'body-sm':     ['0.875rem',  { lineHeight: '1.25rem' }],    // 14px — denser body alt
        'body-md':     ['1rem',      { lineHeight: '1.5rem' }],     // 16px — sub-headings
        // KPI display number. Tabular-nums applied at usage site.
        'display-sm':  ['2rem',      { lineHeight: '1', letterSpacing: '-0.015em', fontWeight: '600' }], // 32px
        'display-md':  ['2.5rem',    { lineHeight: '1', letterSpacing: '-0.02em',  fontWeight: '600' }]  // 40px
      },
      letterSpacing: {
        // Named tracking levels. Replaces tracking-[0.16em] etc.
        eyebrow: '0.14em', // for uppercase eyebrow labels
        caps:    '0.08em'  // milder all-caps
      },
      animation: {
        'fade-in': 'fadeIn 0.2s cubic-bezier(0.16, 1, 0.3, 1)',
        'slide-up': 'slideUp 0.24s cubic-bezier(0.16, 1, 0.3, 1)',
        'slide-down': 'slideDown 0.24s cubic-bezier(0.16, 1, 0.3, 1)',
        'scale-in': 'scaleIn 0.22s cubic-bezier(0.16, 1, 0.3, 1)',
        shimmer: 'shimmer 1.4s linear infinite'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.96)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%':   { backgroundPosition: '-400px 0' },
          '100%': { backgroundPosition: '400px 0' }
        }
      },
      backgroundImage: {
        // Skeleton shimmer gradient — warm-tinted, not pure gray.
        'skeleton-shimmer':
          'linear-gradient(90deg, rgba(231,229,228,0) 0%, rgba(231,229,228,0.6) 50%, rgba(231,229,228,0) 100%)',
        'skeleton-shimmer-dark':
          'linear-gradient(90deg, rgba(41,37,36,0) 0%, rgba(41,37,36,0.7) 50%, rgba(41,37,36,0) 100%)'
      },
      // Page max-width — matches AdminLayout container.
      maxWidth: {
        page: '1500px'
      }
    }
  },
  plugins: []
}
