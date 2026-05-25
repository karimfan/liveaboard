import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // Fail fast instead of falling back to 5174/.../etc. The SPA's
    // /api proxy assumes the dev server is on 5173; a silent
    // fallback would leave the browser hitting the wrong origin.
    // scripts/dev.sh kills any squatter on 5173 before this binds.
    strictPort: true,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: false,
      },
    },
  },
});
