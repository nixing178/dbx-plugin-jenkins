export {};

declare global {
  /** 构建时由 vite define 注入的插件版本号（来源于 package.json）。 */
  const __APP_VERSION__: string;

  interface DbxPluginContext {
    connectionId?: string;
    connectionName?: string;
    databaseType?: string;
    [key: string]: unknown;
  }

  interface DbxPluginTheme {
    appearance: "light" | "dark";
    tokens?: Record<string, string>;
  }

  interface DbxFileSaveTarget {
    handleId: string;
    /** 宿主建议的分块大小（字节）。 */
    chunkBytes?: number;
  }

  /**
   * 桌面宿主的本地文件流式通道；Web 宿主可能整体缺失。
   * 方法名有两套：文档写 beginSave/write/finish/cancel，DBX 0.6.x 实际桥接是
   * beginFileSave/writeFileChunk/finishFileSave/closeFileHandle，调用前先过
   * `normalizeFileTransfer()` 适配，不要直接读某个名字。
   */
  interface DbxFileTransfer {
    beginSave?(request: {
      name: string;
      contentType?: string;
      size?: number;
    }): Promise<DbxFileSaveTarget | null>;
    beginFileSave?(request: {
      name: string;
      contentType?: string;
      size?: number;
    }): Promise<DbxFileSaveTarget | null>;
    write?(
      handleId: string,
      offset: number,
      data: string,
    ): Promise<{ written: number; nextOffset: number } | void>;
    writeFileChunk?(
      handleId: string,
      offset: number,
      data: string,
    ): Promise<{ written: number; nextOffset: number } | void>;
    finish?(handleId: string): Promise<void>;
    finishFileSave?(handleId: string): Promise<void>;
    cancel?(handleId: string): Promise<void>;
    closeFileHandle?(handleId: string): Promise<void>;
  }

  interface DbxPlugin {
    ready: Promise<void>;
    context: DbxPluginContext;
    locale: string;
    theme: DbxPluginTheme;
    capabilities: Record<string, boolean>;
    fileTransfer?: DbxFileTransfer;
    invoke<T = unknown>(
      method: string,
      params?: Record<string, unknown>,
      options?: { timeoutMs?: number },
    ): Promise<T>;
    onEvent?(fn: (event: { method: string; params: unknown }) => void): () => void;
    copy?(text: string): Promise<void> | void;
  }

  interface Window {
    dbxPlugin?: DbxPlugin;
  }
}
