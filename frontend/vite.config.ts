import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "/paipai/",
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 5173,
    strictPort: true,
    proxy: {
      "/paipai/healthz": {
        target: "http://127.0.0.1:18081",
        rewrite: () => "/healthz"
      },
      "/paipai/api/analytics": {
        target: "http://127.0.0.1:18081",
        rewrite: (path) => path.replace(/^\/paipai\/api/, "/v1")
      },
      "/paipai/api/imports": {
        target: "http://127.0.0.1:18081",
        rewrite: (path) => path.replace(/^\/paipai\/api/, "/v1")
      },
      "/paipai/api/xhs-jg/sync": {
        target: "http://127.0.0.1:18080",
        rewrite: (path) => path.replace(/^\/paipai\/api\/xhs-jg/, "/v1")
      },
      "/paipai/api/lark/sync": {
        target: "http://127.0.0.1:18081",
        rewrite: (path) => path.replace(/^\/paipai\/api\/lark/, "/v1")
      }
    }
  }
});
