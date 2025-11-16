import React from 'react';
import { ServifyProvider } from '@servify/react';
import ChatDemo from './components/ChatDemo';

const App: React.FC = () => {
  const handleInitialized = () => {
    console.log('Servify SDK 初始化成功');
  };

  const handleError = (error: Error) => {
    console.error('Servify SDK 错误:', error);
    alert(`SDK 错误: ${error.message}`);
  };

  return (
    <ServifyProvider
      config={{
        apiUrl: 'http://localhost:8080',
        customerName: 'React 用户',
        customerEmail: 'react@example.com',
        debug: true,
      }}
      onInitialized={handleInitialized}
      onError={handleError}
    >
      <div className="container">
        <div className="header">
          <h1>Servify 客服系统</h1>
          <p>React SDK 示例</p>
        </div>
        <ChatDemo />
      </div>
    </ServifyProvider>
  );
};

export default App;