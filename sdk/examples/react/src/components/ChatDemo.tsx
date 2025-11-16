import React, { useState, useRef } from 'react';
import { useServify, useChat, useAI, useSatisfaction } from '@servify/react';

const ChatDemo: React.FC = () => {
  const { isConnected } = useServify();
  const {
    session,
    messages,
    agent,
    isLoading,
    error,
    isAgentTyping,
    startChat,
    sendMessage,
    endChat,
    uploadFile,
  } = useChat();

  const { askAI } = useAI();
  const { submitRating } = useSatisfaction();

  const [messageText, setMessageText] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleSendMessage = async () => {
    if (!messageText.trim() || !session) return;

    try {
      await sendMessage(messageText);
      setMessageText('');
    } catch (err) {
      alert('发送消息失败: ' + (err as Error).message);
    }
  };

  const handleStartChat = async () => {
    try {
      await startChat({
        priority: 'normal',
        message: '您好，我需要帮助',
      });
    } catch (err) {
      alert('开始聊天失败: ' + (err as Error).message);
    }
  };

  const handleEndChat = async () => {
    try {
      await endChat();
    } catch (err) {
      alert('结束聊天失败: ' + (err as Error).message);
    }
  };

  const handleAskAI = async () => {
    const question = prompt('请输入您的问题：');
    if (!question) return;

    try {
      const response = await askAI(question);
      // 添加系统消息显示 AI 回答
      console.log('AI 回答:', response);
    } catch (err) {
      alert('AI 问答失败: ' + (err as Error).message);
    }
  };

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file || !session) return;

    try {
      const result = await uploadFile(file);
      console.log('文件上传成功:', result);
    } catch (err) {
      alert('文件上传失败: ' + (err as Error).message);
    }
  };

  const handleRating = async () => {
    const rating = prompt('请为服务评分 (1-5)：');
    const ratingNum = parseInt(rating || '');

    if (isNaN(ratingNum) || ratingNum < 1 || ratingNum > 5) {
      alert('请输入有效的评分 (1-5)');
      return;
    }

    const comment = prompt('请输入评价内容（可选）：') || '';

    try {
      await submitRating({
        rating: ratingNum,
        comment,
      });
      alert('感谢您的评价！');
    } catch (err) {
      alert('提交评价失败: ' + (err as Error).message);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey && session) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  return (
    <>
      {/* 状态栏 */}
      <div className="status-bar">
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <div className={`status-dot ${isConnected ? 'connected' : 'disconnected'}`}></div>
          <span>{isConnected ? '已连接' : '已断开连接'}</span>
        </div>
        {agent && (
          <div>
            客服代理：<strong>{agent.name}</strong>
          </div>
        )}
      </div>

      {/* 聊天区域 */}
      <div className="chat-area">
        <div className="message system">
          欢迎使用 Servify 客服系统！请开始聊天。
        </div>

        {messages.map((message, index) => (
          <div key={index} className={`message ${message.sender_type}`}>
            {message.content}
          </div>
        ))}

        {error && (
          <div className="message system">
            错误: {error.message}
          </div>
        )}
      </div>

      {/* 输入提示 */}
      {isAgentTyping && (
        <div className="typing-indicator">
          客服正在输入...
        </div>
      )}

      {/* 控制区域 */}
      <div className="controls">
        {/* 消息输入 */}
        <div className="input-group">
          <input
            type="text"
            value={messageText}
            onChange={(e) => setMessageText(e.target.value)}
            onKeyPress={handleKeyPress}
            placeholder="输入您的消息..."
            maxLength={1000}
            disabled={!session}
          />
          <button
            className="btn secondary"
            onClick={() => fileInputRef.current?.click()}
            disabled={!session}
          >
            📎
          </button>
          <button
            className="btn primary"
            onClick={handleSendMessage}
            disabled={!session || !messageText.trim() || isLoading}
          >
            发送
          </button>
        </div>

        {/* 功能按钮 */}
        <div>
          <button
            className="btn success"
            onClick={handleStartChat}
            disabled={!!session || isLoading}
          >
            开始聊天
          </button>
          <button
            className="btn danger"
            onClick={handleEndChat}
            disabled={!session || isLoading}
          >
            结束聊天
          </button>
          <button
            className="btn secondary"
            onClick={handleAskAI}
            disabled={isLoading}
          >
            AI 助手
          </button>
          <button
            className="btn secondary"
            onClick={handleRating}
            disabled={!session}
          >
            评价服务
          </button>
        </div>

        {/* 隐藏的文件输入 */}
        <input
          ref={fileInputRef}
          type="file"
          className="file-input"
          onChange={handleFileUpload}
        />
      </div>
    </>
  );
};

export default ChatDemo;