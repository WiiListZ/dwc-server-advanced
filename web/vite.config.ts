import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

const pages = ["index", "login", "panel", "settings", "errors", "setup", "profile"]

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(import.meta.dirname, "./src") },
  },
  build: {
    // Built pages are embedded into the Go binary by api/panel.go.
    outDir: path.resolve(import.meta.dirname, "../api/dist"),
    emptyOutDir: true,
    assetsDir: "assets",
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      input: Object.fromEntries(pages.map((page) => [page, path.resolve(import.meta.dirname, `${page}.html`)])),
    },
  },
  server: {
    // Run the Go server on :8080 and `npm run dev` for hot reload.
    proxy: { "/api": "http://127.0.0.1:8080" },
  },
})
