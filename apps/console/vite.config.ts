import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

// Local development ONLY: proxy precisely nine fixed read-only Go endpoints.
// No wildcard paths, credentials, or write-capable API operations are proxied.
// Go independently checks the target Host and request method.
export default defineConfig({
  plugins: [react()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '^/api/v1/(health|capabilities|scenarios|proof-summary|control-catalog|environment-snapshot|historical-activity|containment-map/(baseline|hardened))$': {
        target: 'http://127.0.0.1:8787',
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('proxyReq', (outgoing) => {
            // Local API expects no browser authentication and ignores cookies.
            outgoing.removeHeader('cookie');
            outgoing.removeHeader('authorization');
            outgoing.removeHeader('referer');
          });
        },
      },
    },
  },
  preview: { host: '127.0.0.1', port: 4173, strictPort: true },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
});
