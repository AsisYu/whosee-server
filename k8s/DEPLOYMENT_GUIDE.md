# KubeSphere 部署指南 - Whosee-Whois 服务

本文档提供了将 Whosee-Whois 服务部署到 KubeSphere 的详细步骤。

## 前置条件

- 已安装并配置好 KubeSphere
- 拥有 KubeSphere 平台的访问权限
- 拥有 Docker Registry 的访问权限（用于推送镜像）
- 安装了 kubectl 和 Docker 命令行工具

## 部署步骤

### 1. 构建并推送 Docker 镜像

```bash
# 在项目根目录执行
cd /path/to/whosee-whois/server

# 设置环境变量
REGISTRY_ADDR="your-registry-address"  # 例如：docker.io 或 harbor.example.com
REGISTRY_NAMESPACE="your-namespace"   # 例如：your-username 或 your-organization
IMAGE_TAG="latest"                    # 或其他版本标签

# 构建 Docker 镜像
docker build -t ${REGISTRY_ADDR}/${REGISTRY_NAMESPACE}/whosee-whois-server:${IMAGE_TAG} .

# 登录到 Docker Registry
docker login ${REGISTRY_ADDR}

# 推送镜像
docker push ${REGISTRY_ADDR}/${REGISTRY_NAMESPACE}/whosee-whois-server:${IMAGE_TAG}
```

### 2. 准备 Kubernetes 配置文件

在部署前，您需要修改以下配置文件中的相关信息：

1. `deployment.yaml`：替换 `${REGISTRY_ADDR}` 和 `${REGISTRY_NAMESPACE}` 为实际的镜像仓库地址和命名空间
2. `secret.yaml`：更新 base64 编码后的敏感信息（API 密钥、密码等）

对于 `secret.yaml`，您可以使用以下命令生成 base64 编码的值：

```bash
echo -n "your-actual-api-key" | base64
```

### 3. 使用 KubeSphere 部署应用

#### 方式一：通过 KubeSphere Web 控制台

1. 登录 KubeSphere 控制台
2. 创建或选择一个项目（也称为命名空间）
3. 进入「应用负载」→「容器组」，点击「创建」
4. 选择「编排模板」，将 YAML 文件内容粘贴到编辑器中
5. 依次上传所有 YAML 文件（也可以合并为一个文件）
6. 点击「创建」完成部署

#### 方式二：使用 kubectl 命令行

如果您已经配置了 kubectl 访问 KubeSphere 的 Kubernetes 集群，可以直接使用命令行部署：

```bash
# 创建命名空间（如果需要）
kubectl create namespace whosee-whois

# 切换到目标命名空间
kubectl config set-context --current --namespace=whosee-whois

# 应用配置文件
kubectl apply -f k8s/pvc.yaml
kubectl apply -f k8s/redis.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

### 4. 配置网络访问

为了从集群外部访问应用，您需要创建 Ingress（KubeSphere 中称为「应用路由」）：

1. 在 KubeSphere 控制台中，进入您的项目
2. 导航到「应用负载」→「应用路由」
3. 点击「创建」，设置主机名和转发规则
4. 将流量转发到 `whosee-whois-server` 服务的 3000 端口

或者使用 kubectl 创建 Ingress：

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: whosee-whois-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
  - host: whosee-api.example.com  # 替换为您的实际域名
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: whosee-whois-server
            port:
              number: 3000
```

```bash
kubectl apply -f ingress.yaml
```

### 5. 验证部署

部署完成后，可以验证服务是否正常运行：

```bash
# 检查 Pod 状态
kubectl get pods

# 检查服务状态
kubectl get svc

# 查看应用日志
kubectl logs -f deployment/whosee-whois-server
```

通过浏览器访问配置的域名或 IP 地址，检查 API 是否正常响应。

## 故障排除

如果遇到部署问题，请检查：

1. Pod 状态和日志：`kubectl describe pod <pod-name>` 和 `kubectl logs <pod-name>`
2. 服务配置：`kubectl describe service whosee-whois-server`
3. 持久卷声明状态：`kubectl get pvc`

## 维护和更新

### 更新应用

当您需要更新应用时，可以构建新版本镜像并更新部署：

```bash
# 构建新版本镜像
docker build -t ${REGISTRY_ADDR}/${REGISTRY_NAMESPACE}/whosee-whois-server:${NEW_TAG} .
docker push ${REGISTRY_ADDR}/${REGISTRY_NAMESPACE}/whosee-whois-server:${NEW_TAG}

# 更新部署使用新镜像
kubectl set image deployment/whosee-whois-server whosee-whois-server=${REGISTRY_ADDR}/${REGISTRY_NAMESPACE}/whosee-whois-server:${NEW_TAG}
```

或者通过 KubeSphere 控制台更新镜像版本。

### 备份数据

确保定期备份 Redis 数据和应用生成的静态文件：

```bash
# 备份 Redis 数据
kubectl exec -it $(kubectl get pod -l app=whosee-redis -o jsonpath='{.items[0].metadata.name}') -- redis-cli SAVE

# 可以设置定期任务从持久卷中备份数据
```
