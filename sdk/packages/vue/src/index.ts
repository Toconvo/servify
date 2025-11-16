// 导出插件和组合式 API
export { ServifyPlugin, useServify, useServifyReady } from './plugin';
export { useChat, useAI, useTickets, useSatisfaction } from './composables';

// 导出核心类型
export * from '@servify/core';

// 导出类型定义
export type { ServifyPluginOptions } from './plugin';

// 为 Vue 2 兼容性导出
export { ServifyPlugin as default } from './plugin';