# ToHub

ToHub 是一个轻量中转代理，支持 Docker Hub Registry Mirror 和 GitHub 页面代理。

## 功能

- Docker Hub 镜像拉取代理，可配置为 Docker `registry-mirrors`。
- GitHub 页面代理，例如将 `https://github.com/trah01/tohub` 转为 `https://proxy.example.com/trah01/tohub`。
- 提供 Go 服务版本和 Cloudflare Worker 独立版本。

## Cloudflare Worker 部署

点击下面按钮可通过 Cloudflare 一键部署：

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https%3A%2F%2Fgithub.com%2Ftrah01%2Ftohub)

部署完成后，假设地址为：

```text
https://tohub.<your-subdomain>.workers.dev
```

可直接访问首页，或配置 Docker：

```json
{
  "registry-mirrors": ["https://tohub.<your-subdomain>.workers.dev"]
}
```

GitHub 代理示例：

```text
https://tohub.<your-subdomain>.workers.dev/trah01/tohub
https://tohub.<your-subdomain>.workers.dev/github/trah01/tohub
```

### Wrangler 部署

也可以使用 Wrangler 手动部署：

```bash
npm install -g wrangler
wrangler login
wrangler deploy
```

仓库内的 `wrangler.jsonc` 只包含通用部署配置。如果要绑定自定义域名，可在本地 `wrangler.toml` 中配置：

```toml
name = "tohub"
main = "worker.js"
compatibility_date = "2026-07-06"

[[routes]]
pattern = "tohub.example.com"
custom_domain = true
```

## Docker 部署

```bash
docker compose up -d --build
docker compose logs -f tohub
```

访问首页：

```text
http://localhost:8080
```

Docker 配置示例：

```json
{
  "registry-mirrors": ["http://localhost:8080"]
}
```

生产环境建议将 `PUBLIC_BASE_URL` 改为实际访问域名。

## 注意事项

- Docker 代理主要用于拉取镜像，不支持镜像推送。
- GitHub 页面代理会重写页面、静态资源和 Release 下载相关链接，但登录态和接口限制仍可能受上游策略影响。
- 不要在公开服务中记录或暴露认证请求头、Cookie 或 Token。
