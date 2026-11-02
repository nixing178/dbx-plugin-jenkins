<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import CodeEditor from "./CodeEditor.vue";
import { t } from "../i18n";
import {
  DEFAULT_ARTIFACT_ARCHIVE,
  buildCreateJobPayload,
  buildJobUpdateFields,
  validateName,
} from "../lib/forms";
import type {
  ArtifactArchive,
  CreateJobPayload,
  JobConfigResponse,
  JobDetail,
  JobKind,
  JobUpdateFields,
} from "../types";

const props = defineProps<{
  mode: "create" | "edit";
  folder: string;
  config?: JobConfigResponse | null;
  job?: JobDetail | null;
  busy: boolean;
}>();

const emit = defineEmits<{
  create: [payload: CreateJobPayload];
  update: [fields: JobUpdateFields];
  saveXml: [xml: string];
  cancel: [];
}>();

const tab = ref<"form" | "xml">("form");
const name = ref("");
const kind = ref<JobKind>("freestyle");
const title = ref(props.config?.fields.title ?? "");
const description = ref(props.config?.fields.description ?? "");
const disabled = ref(props.config?.fields.disabled ?? false);
const enabled = computed({
  get: () => !disabled.value,
  set: (value: boolean) => {
    disabled.value = !value;
  },
});
const script = ref(props.config?.fields.script ?? "");
const archive = ref<ArtifactArchive>({
  ...DEFAULT_ARTIFACT_ARCHIVE,
  ...(props.config?.fields.archive ?? {}),
});
const xml = ref(props.config?.xml ?? "");
const error = ref("");

const isEdit = computed(() => props.mode === "edit");
const isPipeline = computed(() =>
  isEdit.value ? props.config?.fields.kind === "pipeline" : kind.value === "pipeline",
);

const original = computed(() => ({
  title: props.config?.fields.title ?? "",
  description: props.config?.fields.description ?? "",
  disabled: props.config?.fields.disabled ?? false,
  script: props.config?.fields.script ?? "",
  archive: {
    ...DEFAULT_ARTIFACT_ARCHIVE,
    ...(props.config?.fields.archive ?? {}),
  },
}));

const folderLabel = computed(() => props.folder || t("rootFolderPath"));

function submit(): void {
  // 按钮的 disabled 要等渲染才生效，这里先挡住重复提交（否则第二次必然重名 400）。
  if (props.busy) return;
  error.value = "";
  if (!isEdit.value) {
    const nameError = validateName(name.value);
    if (nameError) {
      error.value = t(nameError);
      return;
    }
    emit(
      "create",
      buildCreateJobPayload({
        folder: props.folder,
        name: name.value,
        kind: kind.value,
        description: description.value,
        script: script.value,
      }),
    );
    return;
  }

  if (tab.value === "xml") {
    if (!xml.value.trim()) {
      error.value = t("xmlRequired");
      return;
    }
    emit("saveXml", xml.value);
    return;
  }

  if (archive.value.enabled && !archive.value.files.trim()) {
    error.value = t("archiveFilesRequired");
    return;
  }

  const fields = buildJobUpdateFields(
    {
      title: title.value,
      description: description.value,
      disabled: disabled.value,
      script: script.value,
      archive: archive.value,
    },
    original.value,
  );
  if (Object.keys(fields).length === 0) {
    emit("cancel");
    return;
  }
  emit("update", fields);
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape") emit("cancel");
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onBeforeUnmount(() => window.removeEventListener("keydown", onKeydown));
</script>

<template>
  <div class="overlay" @click.self="emit('cancel')">
    <section class="modal">
      <header>
        <div>
          <h2>{{ isEdit ? t("editJobTitle") : t("newJobTitle") }}</h2>
          <p class="subtle">
            {{ isEdit ? job?.fullName : t("currentFolder", { path: folderLabel }) }}
          </p>
        </div>
        <button type="button" class="close" @click="emit('cancel')">×</button>
      </header>

      <div v-if="isEdit" class="tabs">
        <button
          type="button"
          :class="{ active: tab === 'form' }"
          @click="tab = 'form'"
        >
          {{ t("basicTab") }}
        </button>
        <button
          type="button"
          :class="{ active: tab === 'xml' }"
          @click="tab = 'xml'"
        >
          {{ t("xmlTab") }}
        </button>
      </div>

      <div class="body">
        <template v-if="!isEdit">
          <label class="field">
            <span class="label">{{ t("jobName") }}</span>
            <input v-model="name" type="text" :placeholder="t('jobName')" autofocus />
          </label>

          <label class="field">
            <span class="label">{{ t("jobType") }}</span>
            <select v-model="kind">
              <option value="freestyle">{{ t("jobKindFreeStyle") }}</option>
              <option value="pipeline">{{ t("jobKindPipeline") }}</option>
            </select>
          </label>

          <label class="field">
            <span class="label">{{ t("description") }}</span>
            <textarea v-model="description" rows="3" />
          </label>

          <div v-if="isPipeline" class="field">
            <span class="label">{{ t("scriptField") }}</span>
            <span class="hint">{{ t("scriptHint") }}</span>
            <CodeEditor v-model="script" :rows="10" />
          </div>
        </template>

        <template v-else-if="tab === 'form'">
          <label class="field">
            <span class="label">{{ t("jobName") }}</span>
            <input v-model="title" type="text" />
            <span class="hint">{{ t("renameHint") }}</span>
          </label>

          <label class="field">
            <span class="label">{{ t("description") }}</span>
            <textarea v-model="description" rows="3" />
          </label>

          <label class="field checkbox-field">
            <input v-model="enabled" type="checkbox" class="checkbox" />
            <span>
              <strong>{{ t("enabledField") }}</strong>
              <span class="hint">{{ t("enabledHint") }}</span>
            </span>
          </label>

          <div v-if="!isPipeline" class="field archive-field">
            <label class="checkbox-field">
              <input v-model="archive.enabled" type="checkbox" class="checkbox" />
              <span>
                <strong>{{ t("archiveArtifactsField") }}</strong>
                <span class="hint">{{ t("archiveArtifactsHint") }}</span>
              </span>
            </label>

            <div v-if="archive.enabled" class="archive-grid">
              <label class="field">
                <span class="label">{{ t("archiveFiles") }}</span>
                <input v-model="archive.files" type="text" placeholder="dist/**/*.jar" />
              </label>

              <label class="field">
                <span class="label">{{ t("archiveExcludes") }}</span>
                <input v-model="archive.excludes" type="text" placeholder="**/*.tmp" />
              </label>

              <div class="archive-flags">
                <label class="checkbox-field">
                  <input v-model="archive.fingerprint" type="checkbox" class="checkbox" />
                  <span>{{ t("archiveFingerprint") }}</span>
                </label>
                <label class="checkbox-field">
                  <input v-model="archive.onlyIfSuccessful" type="checkbox" class="checkbox" />
                  <span>{{ t("archiveOnlyIfSuccessful") }}</span>
                </label>
                <label class="checkbox-field">
                  <input v-model="archive.allowEmptyArchive" type="checkbox" class="checkbox" />
                  <span>{{ t("archiveAllowEmpty") }}</span>
                </label>
              </div>
            </div>
          </div>

          <div v-if="isPipeline" class="field">
            <span class="label">{{ t("scriptField") }}</span>
            <span class="hint">{{ t("scriptHint") }}</span>
            <CodeEditor v-model="script" :rows="12" />
          </div>
        </template>

        <template v-else>
          <p class="hint">{{ t("xmlHint") }}</p>
          <CodeEditor v-model="xml" lang="xml" :rows="16" />
        </template>
      </div>

      <footer>
        <span v-if="error" class="error">{{ error }}</span>
        <button type="button" class="ghost" @click="emit('cancel')">{{ t("cancel") }}</button>
        <button type="button" class="primary" :disabled="busy" @click="submit">
          {{ busy ? t("loading") : isEdit ? t("save") : t("create") }}
        </button>
      </footer>
    </section>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 30;
  display: grid;
  place-items: center;
  padding: 24px;
  background: color-mix(in srgb, #000 42%, transparent);
}

.modal {
  width: min(680px, 100%);
  max-height: min(88vh, 860px);
  display: flex;
  flex-direction: column;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.28);
  overflow: hidden;
}

header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 18px;
  border-bottom: 1px solid var(--border);
}

h2 {
  margin: 0;
  font-size: 16px;
}

.subtle {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
}

.close {
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 22px;
  line-height: 1;
  padding: 0 4px;
}

.close:hover {
  color: var(--text);
}

.tabs {
  display: flex;
  gap: 4px;
  padding: 10px 18px 0;
  border-bottom: 1px solid var(--border);
}

.tabs button {
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--muted);
  padding: 7px 12px 9px;
  font-size: 12px;
  font-weight: 600;
}

.tabs button.active {
  border-bottom-color: var(--accent);
  color: var(--accent);
}

.body {
  padding: 16px 18px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.checkbox-field {
  flex-direction: row;
  align-items: flex-start;
  gap: 10px;
}

.checkbox-field span {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.label {
  font-size: 12px;
  font-weight: 600;
}

.hint {
  color: var(--muted);
  font-size: 11px;
  line-height: 1.5;
}

.note {
  margin: -8px 0 0;
}

.archive-field {
  padding-top: 12px;
  border-top: 1px dashed var(--border);
}

.archive-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
  padding-left: 26px;
}

.archive-flags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 18px;
}



input:not(.checkbox),
select,
textarea {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  background: var(--panel-soft);
  color: var(--text);
  font: inherit;
  font-size: 13px;
  outline: none;
}

input:focus,
select:focus,
textarea:focus {
  border-color: var(--accent);
}

.code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre;
  tab-size: 2;
}

.checkbox {
  width: 16px;
  height: 16px;
  margin-top: 2px;
}

footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 18px;
  border-top: 1px solid var(--border);
}

.error {
  margin-right: auto;
  color: var(--danger);
  font-size: 12px;
}

.ghost,
.primary {
  border-radius: var(--radius-sm);
  padding: 8px 14px;
  font-weight: 600;
}

.ghost {
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
}

.primary {
  border: 0;
  background: var(--accent);
  color: #fff;
}

.primary:disabled {
  opacity: 0.6;
}
</style>
