# Tests Directory

本目录包含项目的测试脚本和测试工具。

## 📁 目录结构

```
tests/
├── README.md                    # 本文件
├── test_runtime.sh              # 旧版运行时测试脚本
└── test_runtime_v2.sh           # 改进版运行时测试脚本（推荐使用）
```

## 🧪 测试脚本说明

### test_runtime_v2.sh（推荐）

**用途**: P0安全修复的自动化运行时测试

**测试覆盖**:
- ✅ .env文件错误处理（不存在 vs 解析错误）
- ✅ Authorization header边界情况（防DoS攻击）
- ✅ IP白名单逻辑（防认证绕过）
- ✅ WHOIS核心功能验证
- ✅ 服务健康检查

**使用方法**:
```bash
# 运行测试
./test_runtime_v2.sh

# 测试会自动：
# 1. 备份.env文件
# 2. 启动测试服务（使用不同端口避免冲突）
# 3. 执行所有测试场景
# 4. 恢复.env文件
# 5. 清理测试进程
```

**要求**:
- Redis运行在localhost:6379
- Go 1.24+
- 端口3901-3903可用

**输出**:
```
========================================
P0修复运行时测试 (改进版)
========================================

[测试] 测试1: .env文件不存在的情况
[通过] 测试1: .env不存在时正确警告并继续运行

[测试] 测试2: .env文件解析错误的情况
[通过] 测试2: .env解析错误时正确fatal终止

...

========================================
测试结果汇总
========================================
✅ 测试1: .env不存在时正确警告并继续运行
✅ 测试2: .env解析错误时正确fatal终止
...

总计: 6 个测试
通过: 6
失败: 0

🎉 所有测试通过！
```

### test_runtime.sh（已废弃）

早期版本的运行时测试脚本，存在端口冲突问题，建议使用`test_runtime_v2.sh`。

## 🔍 未来计划

### 单元测试
```bash
# 为services包添加单元测试
tests/
├── services/
│   ├── whois_manager_test.go  # WhoisManager并发测试
│   ├── circuit_breaker_test.go  # 熔断器测试
│   └── screenshot_service_test.go  # 截图服务测试
```

### Race Detector测试
```bash
# 并发安全验证
go test ./services -race
go build -race -o whosee-race .
```

### 集成测试
```bash
# Docker环境集成测试
tests/
└── integration/
    ├── docker-compose.test.yml
    └── integration_test.sh
```

## 📊 测试报告

所有测试执行报告存放在 `docs/reports/` 目录：
- `RUNTIME_TEST_REPORT.md` - P0修复运行时测试详细报告

## 🚀 CI/CD集成

未来可以将测试脚本集成到CI/CD流程：
```yaml
# .github/workflows/test.yml
- name: Run Runtime Tests
  run: ./tests/test_runtime_v2.sh
```

## 📝 贡献指南

添加新测试时：
1. 使用描述性的测试函数名（`test_xxx`）
2. 添加清晰的日志输出
3. 确保测试独立（不依赖执行顺序）
4. 自动清理测试产生的临时文件
5. 更新本README说明新测试
