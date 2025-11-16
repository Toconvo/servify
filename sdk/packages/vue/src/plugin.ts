import { App, inject, InjectionKey, ref, reactive, onUnmounted } from 'vue';
import { ServifySDK, ServifyConfig } from '@servify/core';

// Vue 3 的注入键
const ServifyKey: InjectionKey<ServifySDK> = Symbol('servify');

// 插件选项
export interface ServifyPluginOptions {
  config: ServifyConfig;
  onInitialized?: () => void;
  onError?: (error: Error) => void;
}

// Vue 插件
export const ServifyPlugin = {
  install(app: App, options: ServifyPluginOptions) {
    const sdk = new ServifySDK(options.config);

    // 初始化 SDK
    sdk.initialize()
      .then(() => {
        options.onInitialized?.();
      })
      .catch((error) => {
        console.error('Failed to initialize Servify SDK:', error);
        options.onError?.(error);
      });

    // 提供 SDK 实例
    app.provide(ServifyKey, sdk);

    // 全局属性
    app.config.globalProperties.$servify = sdk;
  }
};

// 获取 SDK 实例的组合式 API
export function useServify(): ServifySDK {
  const sdk = inject(ServifyKey);
  if (!sdk) {
    throw new Error('Servify SDK not found. Make sure to install the ServifyPlugin.');
  }
  return sdk;
}

// 检查 SDK 是否可用
export function useServifyReady() {
  const sdk = useServify();
  const isReady = ref(false);
  const isConnected = ref(false);

  // 检查连接状态
  const checkConnection = () => {
    isConnected.value = sdk.isConnected();
  };

  // 设置事件监听器
  const handleConnected = () => {
    isReady.value = true;
    isConnected.value = true;
  };

  const handleDisconnected = () => {
    isConnected.value = false;
  };

  sdk.on('connected', handleConnected);
  sdk.on('disconnected', handleDisconnected);

  // 初始检查
  checkConnection();

  // 清理
  onUnmounted(() => {
    sdk.off('connected', handleConnected);
    sdk.off('disconnected', handleDisconnected);
  });

  return {
    isReady: isReady.value,
    isConnected: isConnected.value,
  };
}