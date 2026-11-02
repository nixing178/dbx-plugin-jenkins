<script setup lang="ts">
import { t } from "../i18n";
import { isMultiSelect, parseSelection, toggleSelection } from "../lib/paramValue";
import type { ParameterDefinition } from "../types";

/**
 * 纯参数表单（无遮罩），供单任务构建与批量构建共用。
 * 值用 modelValue 传入，字段变更时整体回传新对象，调用方只管存。
 */
const props = defineProps<{
  parameters: ParameterDefinition[];
  modelValue: Record<string, unknown>;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: Record<string, unknown>];
}>();

function isFile(parameter: ParameterDefinition): boolean {
  return parameter.type === "FileParameterDefinition";
}

function isDisabled(parameter: ParameterDefinition): boolean {
  return isFile(parameter);
}

function textValue(name: string): string {
  const value = props.modelValue[name];
  return value === undefined || value === null ? "" : String(value);
}

function boolValue(name: string): boolean {
  return props.modelValue[name] === true;
}

function update(name: string, value: unknown): void {
  emit("update:modelValue", { ...props.modelValue, [name]: value });
}

function onInput(name: string, event: Event): void {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement;
  update(name, target.value);
}

function onCheck(name: string, event: Event): void {
  update(name, (event.target as HTMLInputElement).checked);
}

function isSelected(parameter: ParameterDefinition, choice: string): boolean {
  return parseSelection(textValue(parameter.name)).includes(choice);
}

function onChoice(parameter: ParameterDefinition, choice: string, event: Event): void {
  const checked = (event.target as HTMLInputElement).checked;
  update(parameter.name, toggleSelection(parameter.choices ?? [], textValue(parameter.name), choice, checked));
}
</script>

<template>
  <div class="param-fields">
    <label v-for="parameter in parameters" :key="parameter.name" class="field">
      <span class="label">
        <code>{{ parameter.name }}</code>
        <span class="type">
          {{ parameter.type.replace("ParameterDefinition", "") || "String" }}
        </span>
      </span>
      <span v-if="parameter.description" class="hint">{{ parameter.description }}</span>
      <span v-if="isFile(parameter)" class="warn">
        {{ t("paramFileUnsupported") }}
      </span>

      <div v-if="isMultiSelect(parameter)" class="choice-group">
        <label v-for="choice in parameter.choices ?? []" :key="choice" class="choice-item">
          <input
            type="checkbox"
            class="checkbox"
            :checked="isSelected(parameter, choice)"
            :disabled="isDisabled(parameter)"
            @change="onChoice(parameter, choice, $event)"
          />
          <span>{{ choice }}</span>
        </label>
      </div>

      <select
        v-else-if="parameter.choices?.length"
        :value="textValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        @change="onInput(parameter.name, $event)"
      >
        <option v-for="choice in parameter.choices ?? []" :key="choice" :value="choice">
          {{ choice }}
        </option>
      </select>

      <textarea
        v-else-if="parameter.type === 'TextParameterDefinition'"
        :value="textValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        rows="3"
        @input="onInput(parameter.name, $event)"
      />

      <input
        v-else-if="parameter.type === 'BooleanParameterDefinition'"
        type="checkbox"
        class="checkbox"
        :checked="boolValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        @change="onCheck(parameter.name, $event)"
      />

      <input
        v-else-if="parameter.type === 'PasswordParameterDefinition'"
        type="password"
        :value="textValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        @input="onInput(parameter.name, $event)"
      />

      <input
        v-else
        type="text"
        :value="textValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        @input="onInput(parameter.name, $event)"
      />
    </label>
  </div>
</template>

<style scoped>
.param-fields {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.label {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
}

.label code {
  font-size: 12px;
  font-weight: 700;
}

.type {
  color: var(--muted);
  font-size: 10px;
}

.hint {
  color: var(--muted);
  font-size: 11px;
  line-height: 1.5;
}

.warn {
  color: #b47d0d;
  font-size: 11px;
  line-height: 1.5;
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

input:disabled,
select:disabled,
textarea:disabled {
  opacity: 0.55;
}

input:focus,
select:focus,
textarea:focus {
  border-color: var(--accent);
}

.checkbox {
  width: 16px;
  height: 16px;
}

.choice-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--panel-soft);
}

.choice-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  cursor: pointer;
}
</style>
