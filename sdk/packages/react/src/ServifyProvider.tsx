import React, { createContext, useContext, useRef, useEffect, ReactNode } from 'react';
import { ServifySDK, ServifyConfig } from '@servify/core';

interface ServifyContextType {
  sdk: ServifySDK | null;
  isInitialized: boolean;
  isConnected: boolean;
}

const ServifyContext = createContext<ServifyContextType>({
  sdk: null,
  isInitialized: false,
  isConnected: false,
});

export interface ServifyProviderProps {
  config: ServifyConfig;
  children: ReactNode;
  onInitialized?: () => void;
  onError?: (error: Error) => void;
}

export function ServifyProvider({
  config,
  children,
  onInitialized,
  onError,
}: ServifyProviderProps): JSX.Element {
  const sdkRef = useRef<ServifySDK | null>(null);
  const [isInitialized, setIsInitialized] = React.useState(false);
  const [isConnected, setIsConnected] = React.useState(false);

  useEffect(() => {
    // 初始化 SDK
    const initSDK = async () => {
      try {
        if (!sdkRef.current) {
          sdkRef.current = new ServifySDK(config);

          // 设置事件监听器
          sdkRef.current.on('connected', () => setIsConnected(true));
          sdkRef.current.on('disconnected', () => setIsConnected(false));
          sdkRef.current.on('error', (error) => onError?.(error));
        }

        await sdkRef.current.initialize();
        setIsInitialized(true);
        onInitialized?.();
      } catch (error) {
        console.error('Failed to initialize Servify SDK:', error);
        onError?.(error as Error);
      }
    };

    initSDK();

    // 清理函数
    return () => {
      if (sdkRef.current) {
        sdkRef.current.disconnect();
        sdkRef.current.removeAllListeners();
      }
    };
  }, [config, onInitialized, onError]);

  const contextValue: ServifyContextType = {
    sdk: sdkRef.current,
    isInitialized,
    isConnected,
  };

  return (
    <ServifyContext.Provider value={contextValue}>
      {children}
    </ServifyContext.Provider>
  );
}

export function useServify(): ServifyContextType {
  const context = useContext(ServifyContext);
  if (!context) {
    throw new Error('useServify must be used within a ServifyProvider');
  }
  return context;
}