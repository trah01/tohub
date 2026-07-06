# HubProxy

HubProxy 是一个用于 Docker Hub 和 GitHub 的轻量中转代理。

## 功能

- Docker Hub Registry Mirror，可配置到 Docker 的 `daemon.json`。
- GitHub 页面代理，可将 `https://github.com/trah01` 转换为 `https://proxy.example.com/trah01`。
- 首页提供 Docker 配置示例和 GitHub 链接转换入口。
- 提供 Go 服务部署版本和 Cloudflare Worker 独立版本。

## Docker Compose 部署

```bash
docker compose up -d --build
```

查看日志：

```bash
docker compose logs -f hubproxy
```

检查状态：

```bash
docker compose ps
curl http://localhost:8080/healthz
```

访问首页：

```text
http://localhost:8080
```

## Docker daemon.json 示例

```json
{
  "registry-mirrors": ["http://localhost:8080"]
}
```

生产环境建议将 `PUBLIC_BASE_URL` 和反向代理域名保持一致，例如：

```yaml
PUBLIC_BASE_URL: "https://proxy.example.com"
```

## GitHub 代理示例

```text
https://proxy.example.com/github
https://github.com/trah01
https://proxy.example.com/trah01
```

也可以使用兼容路径：

```text
https://proxy.example.com/github/trah01
```

首页转换框支持 `github.com/trah01`、`https://github.com/trah01` 和 `trah01/hubproxy`。

GitHub 页面中的静态资源和 Release 下载会通过 `/_hubproxy/<host>/...` 进行中转。该通道仅允许 GitHub 相关域名，避免成为开放代理。

## Cloudflare Worker

`worker.js` 是独立版本，可直接作为 Cloudflare Worker 脚本部署。

Worker 版本同样支持：

- `/v2/` Docker Registry Mirror 请求。
- `/owner/repo` GitHub 页面代理。
- `/` 首页配置说明。

## 注意事项

- Docker 代理主要用于拉取镜像，不支持镜像推送。
- GitHub 页面代理会重写 GitHub 页面、静态资源和 Release 下载相关链接，但登录态、接口限制等仍可能受上游策略影响。
- 不要在公开服务中记录或暴露用户的认证请求头、Cookie 或 Token。
