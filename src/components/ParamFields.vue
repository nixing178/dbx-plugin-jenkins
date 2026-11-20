<script setup lang="ts">
import { t } from "../i18n";
import { isMultiSelect } from "../lib/paramValue";
import type { ParameterDefinition } from "../types";
import MultiSelectField from "./MultiSelectField.vue";

/**
 * 纯参数表单（无遮罩），供单任务构建与批量构建共用。
 * 值用 modelValue 传入，字段变更时整体回传新对象，调用方只管存。
 */
const props = defineProps<{
  parameters: ParameterDefinition[];
  modelValue: Record<string, unknown>;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: Record<string, unknown>];
}>();

function isFile(parameter: ParameterDefinition): boolean {
  return parameter.type === "FileParameterDefinition";
}

function isDisabled(parameter: ParameterDefinition): boolean {
  return isFile(parameter) || props.disabled === true;
}

function isChoiceDisabled(parameter: ParameterDefinition, choice: string): boolean {
  return parameter.disabledChoices?.includes(choice) === true;
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

function updateMultiSelect(parameter: ParameterDefinition, value: string): void {
  update(parameter.name, value);
}
</script>

<template>
  <div class="param-fields">
    <div v-for="(parameter, index) in parameters" :key="parameter.name" class="field">
      <span class="label">
        <label :for="`param-${index}`"><code>{{ parameter.name }}</code></label>
        <span class="type">
          {{ parameter.type.replace("ParameterDefinition", "") || "String" }}
        </span>
      </span>
      <span v-if="parameter.description" class="hint">{{ parameter.description }}</span>
      <span v-if="isFile(parameter)" class="warn">
        {{ t("paramFileUnsupported") }}
      </span>

      <MultiSelectField
        v-if="isMultiSelect(parameter)"
        :name="parameter.name"
        :model-value="textValue(parameter.name)"
        :choices="parameter.choices ?? []"
        :disabled="isDisabled(parameter)"
        :disabled-choices="parameter.disabledChoices ?? []"
        @update:model-value="updateMultiSelect(parameter, $event)"
      />

      <select
        v-else-if="parameter.choices?.length"
        :id="`param-${index}`"
        :value="textValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        @change="onInput(parameter.name, $event)"
      >
        <option
          v-for="choice in parameter.choices ?? []"
          :key="choice"
          :value="choice"
          :disabled="parameter.disabledChoices?.includes(choice)"
        >
          {{ choice }}
        </option>
      </select>

      <textarea
        v-else-if="parameter.type === 'TextParameterDefinition'"
        :id="`param-${index}`"
        :value="textValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        rows="3"
        @input="onInput(parameter.name, $event)"
      />

      <input
        v-else-if="parameter.type === 'BooleanParameterDefinition'"
        :id="`param-${index}`"
        type="checkbox"
        class="checkbox"
        :checked="boolValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        @change="onCheck(parameter.name, $event)"
      />

      <input
        v-else-if="parameter.type === 'PasswordParameterDefinition'"
        :id="`param-${index}`"
        type="password"
        :value="textValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        @input="onInput(parameter.name, $event)"
      />

      <input
        v-else
        :id="`param-${index}`"
        type="text"
        :value="textValue(parameter.name)"
        :disabled="isDisabled(parameter)"
        @input="onInput(parameter.name, $event)"
      />
    </div>
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
</style>
