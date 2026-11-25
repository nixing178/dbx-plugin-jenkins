<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { t } from "../i18n";
import {
  filterParameterChoices,
  nextParameterChoiceIndex,
  selectFilteredParameterChoices,
} from "../lib/multiSelect";
import { parseSelection, toggleSelection } from "../lib/paramValue";

const props = defineProps<{
  name: string;
  modelValue: string;
  choices: string[];
  disabled?: boolean;
  disabledChoices?: string[];
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const open = ref(false);
const query = ref("");
const activeIndex = ref(-1);
const triggerEl = ref<HTMLButtonElement>();
const popoverEl = ref<HTMLDivElement>();
const searchEl = ref<HTMLInputElement>();
const popoverStyle = ref<Record<string, string>>({});
const listId = `multi-select-${props.name.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
const selected = computed(() => new Set(parseSelection(props.modelValue)));
const disabled = computed(() => new Set(props.disabledChoices ?? []));
const isDisabled = computed(() => props.disabled === true);
const selectedLabels = computed(() =>
  props.choices.filter((choice) => selected.value.has(choice)),
);
const filtered = computed(() => filterParameterChoices(props.choices, query.value));

function close(): void {
  if (!open.value) return;
  open.value = false;
  query.value = "";
  activeIndex.value = -1;
  detachListeners();
}

function detachListeners(): void {
  document.removeEventListener("pointerdown", onPointerDown, true);
  document.removeEventListener("focusin", onFocusIn);
  window.removeEventListener("resize", close);
  window.removeEventListener("scroll", onScroll, true);
}

function positionPopover(): void {
  if (!triggerEl.value || !popoverEl.value) return;
  const anchor = triggerEl.value.getBoundingClientRect();
  const height = popoverEl.value.offsetHeight;
  const top = anchor.bottom + height + 8 <= window.innerHeight
    ? anchor.bottom + 6
    : Math.max(6, anchor.top - height - 6);
  popoverStyle.value = {
    top: `${top}px`,
    left: `${anchor.left}px`,
    width: `${anchor.width}px`,
  };
}

async function toggleOpen(): Promise<void> {
  if (props.disabled) return;
  if (open.value) {
    close();
    return;
  }
  open.value = true;
  document.addEventListener("pointerdown", onPointerDown, true);
  document.addEventListener("focusin", onFocusIn);
  window.addEventListener("resize", close);
  window.addEventListener("scroll", onScroll, true);
  activeIndex.value = nextParameterChoiceIndex(filtered.value, -1, 1, props.disabledChoices);
  await nextTick();
  positionPopover();
  searchEl.value?.focus();
}

function onPointerDown(event: PointerEvent): void {
  const target = event.target as Node | null;
  if (target && !triggerEl.value?.contains(target) && !popoverEl.value?.contains(target)) {
    close();
  }
}

function onFocusIn(event: FocusEvent): void {
  const target = event.target as Node | null;
  if (target && !triggerEl.value?.contains(target) && !popoverEl.value?.contains(target)) {
    close();
  }
}

function onScroll(event: Event): void {
  if (!popoverEl.value?.contains(event.target as Node)) close();
}

function onKeydown(event: KeyboardEvent): void {
  if (!open.value) {
    if (["ArrowDown", "ArrowUp", "Enter", " "].includes(event.key)) {
      event.preventDefault();
      event.stopPropagation();
      void toggleOpen();
    }
    return;
  }
  switch (event.key) {
    case "Escape":
      event.preventDefault();
      event.stopPropagation();
      close();
      triggerEl.value?.focus();
      break;
    case "ArrowDown":
    case "ArrowUp": {
      event.preventDefault();
      activeIndex.value = nextParameterChoiceIndex(
        filtered.value,
        activeIndex.value,
        event.key === "ArrowDown" ? 1 : -1,
        props.disabledChoices,
      );
      scrollActiveIntoView();
      break;
    }
    case "Home":
      event.preventDefault();
      activeIndex.value = nextParameterChoiceIndex(filtered.value, -1, 1, props.disabledChoices);
      scrollActiveIntoView();
      break;
    case "End":
      event.preventDefault();
      activeIndex.value = nextParameterChoiceIndex(
        filtered.value,
        filtered.value.length,
        -1,
        props.disabledChoices,
      );
      scrollActiveIntoView();
      break;
    case "Enter":
    case " ": {
      event.preventDefault();
      const choice = filtered.value[activeIndex.value];
      if (choice !== undefined) toggleChoice(choice);
      break;
    }
  }
}

function scrollActiveIntoView(): void {
  void nextTick(() => {
    document.getElementById(`${listId}-${activeIndex.value}`)?.scrollIntoView({ block: "nearest" });
  });
}

function toggleChoice(choice: string): void {
  if (props.disabled || disabled.value.has(choice)) return;
  emit("update:modelValue", toggleSelection(props.choices, props.modelValue, choice, !selected.value.has(choice)));
}

function selectVisible(): void {
  emit("update:modelValue", selectFilteredParameterChoices(
    props.choices,
    filtered.value,
    props.modelValue,
    props.disabledChoices,
  ));
}

function clearSelection(): void {
  emit("update:modelValue", "");
}

function onQueryInput(event: Event): void {
  query.value = (event.target as HTMLInputElement).value;
  activeIndex.value = nextParameterChoiceIndex(filtered.value, -1, 1, props.disabledChoices);
}

watch(() => props.choices, () => {
  close();
});
watch(() => props.disabled, (disabled) => {
  if (disabled) close();
});
onBeforeUnmount(() => detachListeners());
</script>

<template>
  <div class="multi-select">
    <button
      ref="triggerEl"
      type="button"
      class="multi-select-trigger"
      :class="{ placeholder: selectedLabels.length === 0 }"
      :disabled="isDisabled"
      :aria-expanded="open"
      :aria-controls="listId"
      :aria-label="name"
      aria-haspopup="listbox"
      @click="toggleOpen"
      @keydown="onKeydown"
    >
      <span class="multi-select-summary">
        {{ selectedLabels.length ? selectedLabels.join(", ") : t("selectParameterOptions") }}
      </span>
      <span v-if="selectedLabels.length" class="multi-select-count">{{ selectedLabels.length }}</span>
      <svg class="multi-select-chevron" viewBox="0 0 16 16" aria-hidden="true">
        <path d="m4 6 4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.5" />
      </svg>
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        ref="popoverEl"
        class="multi-select-popover"
        :style="popoverStyle"
        @keydown="onKeydown"
      >
        <div class="multi-select-search-wrap">
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <circle cx="7" cy="7" r="4.5" fill="none" stroke="currentColor" />
            <path d="m10.5 10.5 3 3" fill="none" stroke="currentColor" />
          </svg>
          <input
            ref="searchEl"
            type="search"
            :value="query"
            :placeholder="t('filterParameterOptions')"
            :aria-label="t('filterParameterOptions')"
            @input="onQueryInput"
          />
        </div>
        <div class="multi-select-tools">
          <span>{{ t("selectedParameterOptions", { count: selectedLabels.length }) }}</span>
          <div>
            <button type="button" @click="selectVisible">{{ t("selectAll") }}</button>
            <button type="button" :disabled="!selectedLabels.length" @click="clearSelection">
              {{ t("clearSelection") }}
            </button>
          </div>
        </div>
        <div
          :id="listId"
          class="multi-select-options"
          role="listbox"
          :aria-label="name"
          aria-multiselectable="true"
        >
          <button
            v-for="(choice, index) in filtered"
            :id="`${listId}-${index}`"
            :key="choice"
            type="button"
            class="multi-select-option"
            :class="{ active: index === activeIndex, selected: selected.has(choice) }"
            role="option"
            :aria-selected="selected.has(choice)"
            :aria-disabled="disabled.has(choice)"
            :disabled="disabled.has(choice)"
            @mousemove="activeIndex = index"
            @click="toggleChoice(choice)"
          >
            <span class="multi-select-check" aria-hidden="true">{{ selected.has(choice) ? "✓" : "" }}</span>
            <span class="multi-select-option-label" :title="choice">{{ choice }}</span>
          </button>
          <p v-if="!filtered.length" class="multi-select-empty">{{ t("noParameterOptions") }}</p>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.multi-select {
  width: 100%;
}

.multi-select-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 36px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--panel-soft);
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.multi-select-trigger:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
}

.multi-select-trigger:focus-visible,
.multi-select-trigger[aria-expanded="true"] {
  border-color: var(--accent);
  outline: 2px solid var(--accent-soft);
  outline-offset: 1px;
}

.multi-select-trigger:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.multi-select-summary {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.multi-select-trigger.placeholder .multi-select-summary {
  color: var(--muted);
}

.multi-select-count {
  flex: none;
  padding: 0 6px;
  border-radius: 9px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
  line-height: 18px;
}

.multi-select-chevron {
  flex: none;
  width: 16px;
  height: 16px;
  color: var(--muted);
  transition: transform 0.15s;
}

.multi-select-trigger[aria-expanded="true"] .multi-select-chevron {
  transform: rotate(180deg);
}

.multi-select-popover {
  position: fixed;
  z-index: 1200;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: min(360px, calc(100vh - 12px));
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--panel);
  color: var(--text);
  box-shadow: 0 14px 36px rgb(15 23 42 / 0.22);
  font: 13px var(--font);
  text-align: left;
}

.multi-select-search-wrap {
  position: relative;
  flex: none;
}

.multi-select-search-wrap svg {
  position: absolute;
  top: 50%;
  left: 10px;
  width: 16px;
  height: 16px;
  color: var(--muted);
  transform: translateY(-50%);
  pointer-events: none;
}

.multi-select-search-wrap input {
  width: 100%;
  height: 34px;
  padding: 6px 10px 6px 32px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--panel-soft);
  color: var(--text);
  font: inherit;
  outline: none;
}

.multi-select-search-wrap input:focus {
  border-color: var(--accent);
  outline: 2px solid var(--accent-soft);
}

.multi-select-tools {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex: none;
  color: var(--muted);
  font-size: 11px;
}

.multi-select-tools > div {
  display: flex;
  gap: 8px;
}

.multi-select-tools button {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--accent);
  font: inherit;
  cursor: pointer;
}

.multi-select-tools button:disabled {
  color: var(--muted);
  cursor: default;
}

.multi-select-options {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.multi-select-option {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 7px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.multi-select-option.active {
  background: var(--accent-soft);
}

.multi-select-option.selected {
  color: var(--accent);
  font-weight: 600;
}

.multi-select-option:disabled {
  color: var(--muted);
  opacity: 0.55;
  cursor: not-allowed;
}

.multi-select-check {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 16px;
  height: 16px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--panel-soft);
  color: #fff;
  font-size: 11px;
  line-height: 1;
}

.multi-select-option.selected .multi-select-check {
  border-color: var(--accent);
  background: var(--accent);
}

.multi-select-option-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.multi-select-empty {
  margin: 16px 8px;
  color: var(--muted);
  text-align: center;
}
</style>
