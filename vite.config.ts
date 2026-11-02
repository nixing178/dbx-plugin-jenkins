import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import { viteSingleFile } from "vite-plugin-singlefile";
import { version } from "./package.json";

// 版本号从 package.json 注入界面，用于一眼判断当前加载的是哪一版 UI。
// package.json 与 manifest.json 的一致性由 scripts/check-version.mjs 把关。

export default defineConfig({
  base: "./",
  define: {
    __APP_VERSION__: JSON.stringify(version),
  },
  plugins: [vue(), viteSingleFile()],
  build: {
    outDir: "ui",
    emptyOutDir: true,
    target: "es2022",
    assetsInlineLimit: 1024 * 1024,
    chunkSizeWarningLimit: 2048,
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    // 宿主 FS shim 偶发拦截并行 worker 的临时 SSR 文件，串行执行保证稳定。
    fileParallelism: false,
  },
});
