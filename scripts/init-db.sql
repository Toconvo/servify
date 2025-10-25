# 数据库初始化脚本

-- 创建 pgvector 扩展
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS hstore;

-- 创建 WeKnora 与 Servify 的映射表
CREATE TABLE IF NOT EXISTS servify_weknora_mappings (
    id SERIAL PRIMARY KEY,
    servify_doc_id INTEGER,
    weknora_doc_id VARCHAR(255) NOT NULL,
    weknora_kb_id VARCHAR(255) NOT NULL,
    mapping_type VARCHAR(50) DEFAULT 'document',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(servify_doc_id, weknora_doc_id)
);

-- 创建索引优化查询
CREATE INDEX IF NOT EXISTS idx_mappings_servify_doc ON servify_weknora_mappings(servify_doc_id);
CREATE INDEX IF NOT EXISTS idx_mappings_weknora_doc ON servify_weknora_mappings(weknora_doc_id);
CREATE INDEX IF NOT EXISTS idx_mappings_kb ON servify_weknora_mappings(weknora_kb_id);

-- 创建 AI 服务监控表
CREATE TABLE IF NOT EXISTS ai_service_metrics (
    id SERIAL PRIMARY KEY,
    service_type VARCHAR(50) NOT NULL, -- 'weknora', 'fallback', 'openai'
    query_count BIGINT DEFAULT 0,
    success_count BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    avg_latency_ms INTEGER DEFAULT 0,
    last_updated TIMESTAMP DEFAULT NOW(),
    date DATE DEFAULT CURRENT_DATE,
    UNIQUE(service_type, date)
);

-- 创建知识库同步日志表
CREATE TABLE IF NOT EXISTS knowledge_sync_logs (
    id SERIAL PRIMARY KEY,
    operation VARCHAR(50) NOT NULL, -- 'upload', 'update', 'delete'
    document_id VARCHAR(255),
    servify_doc_id INTEGER,
    weknora_doc_id VARCHAR(255),
    status VARCHAR(50) NOT NULL, -- 'pending', 'success', 'failed'
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP
);

-- 插入初始指标数据
INSERT INTO ai_service_metrics (service_type, date)
VALUES
    ('weknora', CURRENT_DATE),
    ('fallback', CURRENT_DATE),
    ('openai', CURRENT_DATE)
ON CONFLICT (service_type, date) DO NOTHING;

-- 创建更新时间戳的函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 创建触发器
CREATE TRIGGER update_mappings_updated_at
    BEFORE UPDATE ON servify_weknora_mappings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 创建一些示例数据（开发环境）
INSERT INTO knowledge_docs (title, content, category, tags, created_at, updated_at)
VALUES
    ('产品安装指南', '本指南详细介绍了产品的安装步骤...', '技术支持', '安装,指南,技术', NOW(), NOW()),
    ('常见问题解答', '以下是用户最常遇到的问题及解决方案...', '常见问题', '问题,解答,FAQ', NOW(), NOW()),
    ('API 使用文档', 'REST API 接口说明和使用示例...', '开发文档', 'API,开发,文档', NOW(), NOW())
ON CONFLICT DO NOTHING;