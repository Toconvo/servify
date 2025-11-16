// 导出 Provider 和主要 Hook
export { ServifyProvider, useServify } from './ServifyProvider';
export { useChat } from './useChat';
export { useTickets, useAI, useSatisfaction } from './hooks';

// 导出核心类型
export * from '@servify/core';

// 导出类型定义
export type { ServifyProviderProps } from './ServifyProvider';
export type {
  UseChatReturn,
  UseTicketsReturn,
  UseAIReturn,
  UseSatisfactionReturn,
} from './hooks';