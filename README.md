# ToHub

ToHub 是一个用于 Docker Hub 和 GitHub 的轻量中转代理。

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
docker compose logs -f tohub
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

首页转换框支持 `github.com/trah01`、`https://github.com/trah01` 和 `trah01/tohub`。

GitHub 页面中的静态资源和 Release 下载会通过 `/_tohub/<host>/...` 进行中转。该通道仅允许 GitHub 相关域名，避免成为开放代理。

## Cloudflare Worker

`worker.js` 是独立版本，可直接作为 Cloudflare Worker 脚本部署，不依赖 Go 服务、Docker 或额外存储。

Worker 版本同样支持：

- `/v2/` Docker Registry Mirror 请求。
- `/owner/repo` GitHub 页面代理。
- `/` 首页配置说明。

### 方式一：使用 Wrangler 部署

安装并登录 Wrangler：

```bash
npm install -g wrangler
wrangler login
```

在项目根目录创建 `wrangler.toml`：

```toml
name = "tohub"
main = "worker.js"
compatibility_date = "2026-07-06"
```

部署：

```bash
wrangler deploy
```

部署成功后，Wrangler 会输出一个 `*.workers.dev` 地址。假设地址是：

```text
https://tohub.<your-subdomain>.workers.dev
```

可以这样检查服务：

```bash
curl https://tohub.<your-subdomain>.workers.dev/healthz
curl -I https://tohub.<your-subdomain>.workers.dev/v2/
```

浏览器访问首页：

```text
https://tohub.<your-subdomain>.workers.dev
```

GitHub 代理示例：

```text
https://tohub.<your-subdomain>.workers.dev/trah01/tohub
https://tohub.<your-subdomain>.workers.dev/github/trah01/tohub
```

Docker Registry Mirror 示例：

```json
{
  "registry-mirrors": ["https://tohub.<your-subdomain>.workers.dev"]
}
```

修改 Docker 配置后需要重启 Docker 服务，再继续使用原有的 `docker pull` 命令。

### 方式二：绑定自定义域名

如果域名已经接入 Cloudflare，可以在 `wrangler.toml` 中增加自定义域名配置：

```toml
name = "tohub"
main = "worker.js"
compatibility_date = "2026-07-06"

[[routes]]
pattern = "tohub.example.com"
custom_domain = true
```

然后重新部署：

```bash
wrangler deploy
```

部署完成后，将 Docker mirror 地址和 GitHub 代理地址改为你的自定义域名：

```json
{
  "registry-mirrors": ["https://tohub.example.com"]
}
```

```text
https://tohub.example.com/trah01/tohub
```

### Cloudflare 控制台部署

也可以在 Cloudflare 控制台手动创建 Worker：

1. 进入 Workers & Pages。
2. 创建 Worker。
3. 将 `worker.js` 的内容粘贴到在线编辑器。
4. 保存并部署。
5. 在 Settings 或 Triggers 中绑定 `workers.dev` 或自定义域名。

控制台方式适合快速测试；长期维护建议使用 Wrangler，以便通过 Git 管理脚本变更。

## 注意事项

- Docker 代理主要用于拉取镜像，不支持镜像推送。
- GitHub 页面代理会重写 GitHub 页面、静态资源和 Release 下载相关链接，但登录态、接口限制等仍可能受上游策略影响。
- 不要在公开服务中记录或暴露用户的认证请求头、Cookie 或 Token。
