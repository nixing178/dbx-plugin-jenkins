import { createApp, h } from "vue";
import BuildView from "./components/BuildView.vue";
import { locale } from "./i18n";
import type { BuildDetail } from "./types";
import "./style.css";

/**
 * 构建详情页「构建参数」预览：只用于开发时肉眼检查网格换行、长值折行与空值占位。
 * `pnpm dev` 打开 /parameters-preview.html；`?theme=dark` 切深色。
 */
locale.value = "zh";
const theme = new URLSearchParams(location.search).get("theme");
if (theme === "dark") document.documentElement.dataset.theme = "dark";

const now = Date.now();

const build: BuildDetail = {
  number: 131,
  result: "SUCCESS",
  building: false,
  timestamp: now - 18 * 60_000,
  duration: 268_000,
  estimatedDuration: 260_000,
  displayName: "#131",
  fullDisplayName: "Builds » demo-schedule-service #131",
  url: "https://jenkins.example/job/Builds/job/demo-schedule-service/131/",
  causes: [{ shortDescription: "Started by user 源哥" }],
  status: "success",
  parameters: [
    { name: "ENV", value: "prod" },
    { name: "BRANCH_NAME", value: "release/2026.11" },
    { name: "MODULE_NAME", value: "demo-schedule-service" },
    { name: "SKIP_TESTS", value: "false" },
    { name: "STATUS", value: "READY_FOR_RELEASE" },
    { name: "DEPLOY_TARGETS", value: "cn-hangzhou,cn-shanghai,ap-southeast-1" },
    {
      name: "MAVEN_ARGS",
      value: "-DskipITs -Dmaven.test.failure.ignore=false -Dfile.encoding=UTF-8 -Duser.timezone=Asia/Shanghai",
    },
    { name: "NOTES", value: "" },
  ],
  changeSet: [],
  artifacts: [],
};

const refresh = {
  mode: "build",
  paused: false,
  inFlight: false,
  lastAt: Date.now(),
  intervalMs: 3000,
  refreshing: false,
  refreshed: false,
};

const Harness = {
  setup() {
    return () =>
      h(
        "div",
        { style: "height:100vh;display:flex;background:var(--bg);font-family:var(--font)" },
        [
          h(BuildView, {
            build,
            jobFullName: "Builds/demo-schedule-service",
            readOnly: true,
            locale: "zh",
            refresh,
          }),
        ],
      );
  },
};

createApp(Harness).mount("#app");
