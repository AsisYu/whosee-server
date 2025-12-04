#!/bin/bash

# 运行时测试脚本 - P0修复验证
# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

TEST_RESULTS=()

# 辅助函数
log_test() {
    echo -e "${YELLOW}[测试] $1${NC}"
}

log_pass() {
    echo -e "${GREEN}[通过] $1${NC}"
    TEST_RESULTS+=("✅ $1")
}

log_fail() {
    echo -e "${RED}[失败] $1${NC}"
    TEST_RESULTS+=("❌ $1")
}

# 测试1: .env文件不存在（应该只是警告）
test_env_missing() {
    log_test "测试1: .env文件不存在的情况"

    rm -f .env
    timeout 5s go run main.go > /tmp/test_output_1.log 2>&1 &
    PID=$!
    sleep 2

    if ps -p $PID > /dev/null 2>&1; then
        kill $PID 2>/dev/null
        if grep -q "未找到.env文件" /tmp/test_output_1.log; then
            log_pass "测试1: .env不存在时正确警告并继续运行"
        else
            log_fail "测试1: 未看到预期的警告信息"
        fi
    else
        log_fail "测试1: 服务意外终止"
    fi
}

# 测试2: .env文件解析错误（应该fatal）
test_env_parse_error() {
    log_test "测试2: .env文件解析错误的情况"

    echo "INVALID LINE WITHOUT EQUALS" > .env
    timeout 5s go run main.go > /tmp/test_output_2.log 2>&1 &
    PID=$!
    sleep 2

    if ! ps -p $PID > /dev/null 2>&1; then
        if grep -q "加载.env文件失败" /tmp/test_output_2.log; then
            log_pass "测试2: .env解析错误时正确fatal终止"
        else
            log_fail "测试2: 未看到预期的fatal错误"
        fi
    else
        kill $PID 2>/dev/null
        log_fail "测试2: 服务应该终止但仍在运行"
    fi
}

# 测试3: Authorization header边界情况
test_auth_header() {
    log_test "测试3: Authorization header边界情况"

    # 恢复.env
    cp .env.backup .env 2>/dev/null

    # 启动服务
    export API_DEV_MODE=true
    export REDIS_ADDR=localhost:6379
    export JWT_SECRET=test_secret_key_12345
    export API_KEY=test_api_key

    go run main.go > /tmp/server.log 2>&1 &
    SERVER_PID=$!
    sleep 5

    # 生成token
    TOKEN=$(curl -s http://localhost:3900/api/auth/token | grep -o '"token":"[^"]*"' | sed 's/"token":"//;s/"//')

    if [ -z "$TOKEN" ]; then
        kill $SERVER_PID 2>/dev/null
        log_fail "测试3: 无法获取token"
        return
    fi

    # 测试3a: 带空格的Bearer header
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization:  Bearer $TOKEN  " http://localhost:3900/api/v1/whois/google.com)
    if [ "$RESPONSE" == "200" ] || [ "$RESPONSE" == "403" ]; then
        log_pass "测试3a: 带空格的Authorization header被正确处理"
    else
        log_fail "测试3a: 带空格的header返回$RESPONSE"
    fi

    # 测试3b: 错误的大小写（bearer小写，应该被拒绝）
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: bearer $TOKEN" http://localhost:3900/api/v1/whois/google.com)
    if [ "$RESPONSE" == "401" ]; then
        log_pass "测试3b: 小写bearer被正确拒绝（RFC 6750合规）"
    else
        log_fail "测试3b: 小写bearer应该返回401，实际返回$RESPONSE"
    fi

    # 测试3c: 短字符串不会panic
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: x" http://localhost:3900/api/v1/whois/google.com)
    if [ "$RESPONSE" == "401" ]; then
        log_pass "测试3c: 短Authorization header被正确拒绝（无panic）"
    else
        log_fail "测试3c: 短header应该返回401，实际返回$RESPONSE"
    fi

    kill $SERVER_PID 2>/dev/null
}

# 测试4: IP白名单+API Key组合逻辑
test_ip_api_key_logic() {
    log_test "测试4: IP白名单+API Key组合逻辑"

    # 恢复.env
    cp .env.backup .env 2>/dev/null

    # 非严格模式测试
    export API_DEV_MODE=false
    export IP_WHITELIST_STRICT_MODE=false
    export REDIS_ADDR=localhost:6379
    export API_KEY=test_api_key_12345
    export JWT_SECRET=test_secret_key_12345

    go run main.go > /tmp/server2.log 2>&1 &
    SERVER_PID=$!
    sleep 5

    # 测试4a: 本地IP无API Key（应该通过，因为本地IP在白名单）
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3900/api/health)
    if [ "$RESPONSE" == "200" ]; then
        log_pass "测试4a: 本地IP无API Key在非严格模式下通过"
    else
        log_fail "测试4a: 本地IP应该通过，实际返回$RESPONSE"
    fi

    # 测试4b: 用API Key访问
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: test_api_key_12345" http://localhost:3900/api/v1/whois/google.com)
    if [ "$RESPONSE" == "200" ] || [ "$RESPONSE" == "500" ]; then
        log_pass "测试4b: API Key认证通过"
    else
        log_fail "测试4b: API Key应该通过，实际返回$RESPONSE"
    fi

    # 清理Redis缓存，测试缓存分离
    redis-cli DEL "ip:check:127.0.0.1" > /dev/null 2>&1

    # 测试4c: 验证缓存不会导致认证绕过（先用API Key，再不用）
    curl -s -H "X-API-KEY: test_api_key_12345" http://localhost:3900/api/v1/whois/google.com > /dev/null
    sleep 1
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3900/api/v1/whois/google.com)
    if [ "$RESPONSE" == "200" ]; then
        log_pass "测试4c: IP白名单允许访问（不依赖API Key缓存）"
    else
        log_fail "测试4c: 本地IP应该在白名单，实际返回$RESPONSE"
    fi

    kill $SERVER_PID 2>/dev/null
}

# 测试5: WHOIS核心功能
test_whois_core() {
    log_test "测试5: WHOIS核心功能"

    export API_DEV_MODE=true
    export REDIS_ADDR=localhost:6379

    go run main.go > /tmp/server3.log 2>&1 &
    SERVER_PID=$!
    sleep 5

    # 测试WHOIS查询
    RESPONSE=$(curl -s http://localhost:3900/api/v1/whois/google.com)
    if echo "$RESPONSE" | grep -q "domain"; then
        log_pass "测试5: WHOIS核心功能正常工作"
    else
        log_fail "测试5: WHOIS查询失败"
    fi

    kill $SERVER_PID 2>/dev/null
}

# 执行所有测试
echo "========================================"
echo "P0修复运行时测试"
echo "========================================"
echo ""

test_env_missing
echo ""

test_env_parse_error
echo ""

test_auth_header
echo ""

test_ip_api_key_logic
echo ""

test_whois_core
echo ""

# 恢复.env
cp .env.backup .env 2>/dev/null
echo "已恢复原始.env文件"
echo ""

# 输出测试结果
echo "========================================"
echo "测试结果汇总"
echo "========================================"
for result in "${TEST_RESULTS[@]}"; do
    echo "$result"
done
echo ""

# 统计
PASSED=$(echo "${TEST_RESULTS[@]}" | grep -o "✅" | wc -l)
FAILED=$(echo "${TEST_RESULTS[@]}" | grep -o "❌" | wc -l)
TOTAL=$((PASSED + FAILED))

echo "总计: $TOTAL 个测试"
echo -e "${GREEN}通过: $PASSED${NC}"
echo -e "${RED}失败: $FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}🎉 所有测试通过！${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}⚠️ 有测试失败，请检查！${NC}"
    exit 1
fi
