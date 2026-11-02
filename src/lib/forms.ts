import type { ArtifactArchive } from "../types";

export type JobKind = "freestyle" | "pipeline";

export type NameErrorKey = "nameRequired" | "nameInvalid";

const MAX_NAME_LENGTH = 255;
// Jenkins NameValidator 拒绝的字符（2.586 实测），其中 " 会让 Jenkins 直接返回 500。
const INVALID_NAME_CHARS = /[/\\?#%*:<>|&;!@$^\[\]"]/;

/** 与后端 ValidateName 保持一致：前端先拦一层，后端仍会二次校验。 */
export function validateName(name: string): NameErrorKey | null {
  const trimmed = name.trim();
  if (!trimmed) return "nameRequired";
  if (Array.from(trimmed).length > MAX_NAME_LENGTH) return "nameInvalid";
  if (trimmed.includes("..")) return "nameInvalid";
  if (INVALID_NAME_CHARS.test(trimmed)) return "nameInvalid";
  for (const char of trimmed) {
    const code = char.codePointAt(0) ?? 0;
    if (code < 0x20 || code === 0x7f) return "nameInvalid";
  }
  return null;
}

export interface CreateJobInput {
  folder: string;
  name: string;
  kind: JobKind;
  description: string;
  script: string;
}

export interface CreateJobPayload {
  folder: string;
  name: string;
  kind: JobKind;
  description: string;
  script: string;
}

export function buildCreateJobPayload(input: CreateJobInput): CreateJobPayload {
  return {
    folder: input.folder.trim(),
    name: input.name.trim(),
    kind: input.kind,
    description: input.description.trim(),
    script: input.kind === "pipeline" ? input.script.trim() : "",
  };
}

export interface CreateViewInput {
  name: string;
  description: string;
  jobs: string[];
  includeRegex: string;
  recurse: boolean;
}

export interface CreateViewPayload {
  name: string;
  description: string;
  jobs: string[];
  includeRegex: string;
  recurse: boolean;
}

export function buildCreateViewPayload(input: CreateViewInput): CreateViewPayload {
  return {
    name: input.name.trim(),
    description: input.description.trim(),
    jobs: uniqueList(input.jobs),
    includeRegex: input.includeRegex.trim(),
    recurse: input.recurse,
  };
}

export interface JobFormValues {
  title: string;
  description: string;
  disabled: boolean;
  script: string;
  archive: ArtifactArchive;
}

export interface JobUpdateFields {
  title?: string;
  description?: string;
  disabled?: boolean;
  script?: string;
  archive?: ArtifactArchive;
}

/** 归档产物的默认值：关闭状态，defaultExcludes 与 Jenkins 一致为 true。 */
export const DEFAULT_ARTIFACT_ARCHIVE: ArtifactArchive = {
  enabled: false,
  files: "",
  excludes: "",
  fingerprint: false,
  onlyIfSuccessful: false,
  allowEmptyArchive: false,
  defaultExcludes: true,
  caseSensitive: false,
};

/** 只提交发生变化的字段，避免无谓地重写 config.xml。 */
export function buildJobUpdateFields(
  current: JobFormValues,
  original: JobFormValues,
): JobUpdateFields {
  const fields: JobUpdateFields = {};
  const title = current.title.trim();
  if (title && title !== original.title.trim()) fields.title = title;
  if (current.description !== original.description) {
    fields.description = current.description;
  }
  if (current.disabled !== original.disabled) fields.disabled = current.disabled;
  if (current.script !== original.script) fields.script = current.script;
  if (!sameArchive(current.archive, original.archive)) {
    fields.archive = normalizeArchive(current.archive);
  }
  return fields;
}

function normalizeArchive(input: ArtifactArchive): ArtifactArchive {
  return {
    ...input,
    files: input.files.trim(),
    excludes: input.excludes.trim(),
  };
}

/** 归档配置逐字段比较；文件模式与排除按 trim 后比较，避免空格触发无谓写入。 */
function sameArchive(left: ArtifactArchive, right: ArtifactArchive): boolean {
  return (
    left.enabled === right.enabled &&
    left.files.trim() === right.files.trim() &&
    left.excludes.trim() === right.excludes.trim() &&
    left.fingerprint === right.fingerprint &&
    left.onlyIfSuccessful === right.onlyIfSuccessful &&
    left.allowEmptyArchive === right.allowEmptyArchive &&
    left.defaultExcludes === right.defaultExcludes &&
    left.caseSensitive === right.caseSensitive
  );
}

export interface ViewFormValues {
  name: string;
  description: string;
  jobNames: string[];
  includeRegex: string;
  recurse: boolean;
}

export interface ViewUpdateFields {
  name?: string;
  description?: string;
  jobNames?: string[];
  includeRegex?: string;
  recurse?: boolean;
}

export function buildViewUpdateFields(
  current: ViewFormValues,
  original: ViewFormValues,
): ViewUpdateFields {
  const fields: ViewUpdateFields = {};
  const name = (current.name ?? "").trim();
  // 空名表示「不修改」；同名不发请求，Jenkins 那边也是 noop。
  if (name && name !== original.name.trim()) fields.name = name;
  if (current.description !== original.description) {
    fields.description = current.description;
  }
  if (!sameSet(current.jobNames, original.jobNames)) {
    fields.jobNames = uniqueList(current.jobNames);
  }
  const regex = current.includeRegex.trim();
  if (regex !== original.includeRegex.trim()) fields.includeRegex = regex;
  if (current.recurse !== original.recurse) fields.recurse = current.recurse;
  return fields;
}

export function splitJobNames(text: string): string[] {
  return uniqueList(text.split(/[,\n]/));
}

export function deleteConfirmationMatches(input: string, target: string): boolean {
  return input.trim() === target.trim();
}

/** trim + 去空 + 保序去重。 */
export function uniqueList(values: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const value of values) {
    const trimmed = value.trim();
    if (!trimmed || seen.has(trimmed)) continue;
    seen.add(trimmed);
    result.push(trimmed);
  }
  return result;
}

function sameSet(left: string[], right: string[]): boolean {
  const a = uniqueList(left).sort();
  const b = uniqueList(right).sort();
  if (a.length !== b.length) return false;
  return a.every((value, index) => value === b[index]);
}
