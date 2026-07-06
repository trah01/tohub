const DOCKER_REGISTRY = "https://registry-1.docker.io";
const DOCKER_AUTH = "https://auth.docker.io/token";
const GITHUB_HOSTS = new Set([
  "github.com",
  "api.github.com",
  "gist.github.com",
  "github.githubassets.com",
  "avatars.githubusercontent.com",
  "raw.githubusercontent.com",
  "objects.githubusercontent.com",
  "user-images.githubusercontent.com",
  "camo.githubusercontent.com"
]);

export default {
  async fetch(request) {
    const url = new URL(request.url);
    if (url.pathname === "/healthz") {
      return json({ status: "ok" });
    }
    if (url.pathname === "/" && request.method === "GET") {
      return new Response(indexHTML(url.origin), {
        headers: { "content-type": "text/html; charset=utf-8" }
      });
    }
    if (url.pathname.startsWith("/v2/")) {
      return dockerProxy(request, url);
    }
    return githubProxy(request, url);
  }
};

async function dockerProxy(request, url) {
  if (url.pathname === "/v2/" || url.pathname === "/v2") {
    return new Response(null, {
      status: 200,
      headers: { "Docker-Distribution-API-Version": "registry/2.0" }
    });
  }
  if (!["GET", "HEAD"].includes(request.method)) {
    return new Response("docker mirror only supports pull requests", { status: 405 });
  }
  const repository = dockerRepository(url.pathname);
  if (!repository) {
    return new Response("invalid docker registry path", { status: 400 });
  }

  const upstreamURL = new URL(url.pathname + url.search, DOCKER_REGISTRY);
  let response = await fetch(upstreamURL, copyRequest(request));
  if (response.status === 401) {
    const token = await dockerToken(repository);
    const retry = copyRequest(request);
    retry.headers.set("Authorization", `Bearer ${token}`);
    response = await fetch(upstreamURL, retry);
  }
  return withDockerHeader(response);
}

function dockerRepository(pathname) {
  const parts = pathname.replace(/^\/v2\//, "").split("/");
  const index = parts.findIndex((part) => ["manifests", "blobs", "tags"].includes(part));
  if (index <= 0) {
    return "";
  }
  return parts.slice(0, index).join("/");
}

async function dockerToken(repository) {
  const url = new URL(DOCKER_AUTH);
  url.searchParams.set("service", "registry.docker.io");
  url.searchParams.set("scope", `repository:${repository}:pull`);
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`docker token status ${response.status}`);
  }
  const payload = await response.json();
  return payload.token || payload.access_token;
}

function withDockerHeader(response) {
  const headers = new Headers(response.headers);
  headers.set("Docker-Distribution-API-Version", "registry/2.0");
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers
  });
}

async function githubProxy(request, url) {
  if (!["GET", "HEAD"].includes(request.method)) {
    return new Response("github proxy only supports read requests", { status: 405 });
  }
  const upstream = githubTarget(url);
  if (!upstream) {
    return new Response("unsupported github upstream host", { status: 400 });
  }
  const response = await fetch(upstream, copyRequest(request));
  const headers = new Headers(response.headers);
  headers.delete("content-security-policy");
  headers.delete("content-security-policy-report-only");
  if (headers.has("location")) {
    headers.set("location", rewriteURL(headers.get("location"), url.origin));
  }
  const contentType = headers.get("content-type") || "";
  if (!shouldRewriteBody(contentType, upstream.hostname)) {
    return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
  }
  const text = await response.text();
  headers.delete("content-length");
  return new Response(rewriteBody(text, url.origin, contentType, upstream.hostname), {
    status: response.status,
    statusText: response.statusText,
    headers
  });
}

function githubTarget(url) {
  let host = "github.com";
  let pathname = url.pathname;
  if (pathname.startsWith("/_tohub/") || pathname.startsWith("/_hubproxy/")) {
    const rest = pathname.replace(/^\/_(tohub|hubproxy)\//, "");
    const slashIndex = rest.indexOf("/");
    host = slashIndex >= 0 ? rest.slice(0, slashIndex) : rest;
    pathname = slashIndex >= 0 ? rest.slice(slashIndex) : "/";
    if (!GITHUB_HOSTS.has(host)) {
      return null;
    }
  } else {
    pathname = githubPath(pathname);
  }
  return new URL(pathname + url.search, `https://${host}`);
}

function githubPath(pathname) {
  if (pathname === "/github" || pathname === "/github/") {
    return "/";
  }
  if (pathname.startsWith("/github/")) {
    return pathname.replace(/^\/github/, "");
  }
  return pathname;
}

function rewriteHTML(html, origin) {
  let rewritten = html
    .replaceAll("https://github.com", origin)
    .replaceAll("http://github.com", origin)
    .replaceAll("//github.com", origin);
  for (const host of GITHUB_HOSTS) {
    if (host === "github.com") {
      continue;
    }
    const proxied = `${origin}/_tohub/${host}`;
    rewritten = rewritten
      .replaceAll(`https://${host}`, proxied)
      .replaceAll(`http://${host}`, proxied)
      .replaceAll(`//${host}`, proxied);
  }
  return rewritten;
}

function shouldRewriteBody(contentType, host) {
  return contentType.includes("text/html") ||
    contentType.includes("text/css") ||
    (contentType.includes("javascript") && host === "github.githubassets.com");
}

function rewriteBody(text, origin, contentType, host) {
  if (contentType.includes("text/html")) {
    return rewriteHTML(text, origin);
  }
  if (host === "github.githubassets.com") {
    const proxiedAssets = `${origin}/_tohub/${host}/assets/`;
    return text
      .replaceAll("\"/assets/", `"${proxiedAssets}`)
      .replaceAll("'/assets/", `'${proxiedAssets}`)
      .replaceAll("(/assets/", `(${proxiedAssets}`);
  }
  return text;
}

function rewriteURL(value, origin) {
  try {
    const parsed = new URL(value);
    if (!GITHUB_HOSTS.has(parsed.hostname)) {
      return value;
    }
    if (parsed.hostname === "github.com") {
      parsed.protocol = "https:";
      parsed.host = new URL(origin).host;
      return parsed.toString();
    }
    return `${origin}/_tohub/${parsed.hostname}${parsed.pathname}${parsed.search}${parsed.hash}`;
  } catch (_) {
    return value;
  }
}

function copyRequest(request) {
  const headers = new Headers(request.headers);
  headers.delete("host");
  return {
    method: request.method,
    headers,
    redirect: "follow"
  };
}

function json(value) {
  return new Response(JSON.stringify(value), {
    headers: { "content-type": "application/json; charset=utf-8" }
  });
}

function indexHTML(origin) {
  const dockerConfig = JSON.stringify({ "registry-mirrors": [origin] }, null, 2);
  return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ToHub</title>
  <style>
    body{margin:0;background:#f7f8fa;color:#16181d;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
    main{width:min(920px,calc(100% - 32px));margin:0 auto;padding:56px 0}
    h1{font-size:56px;line-height:1;margin:0 0 12px;letter-spacing:0}
    h2{font-size:20px;margin:0 0 16px;letter-spacing:0}
    p{color:#5c6470;line-height:1.7}
    section{margin-top:18px;padding:24px;background:#fff;border:1px solid #d9dde4;border-radius:8px;box-shadow:0 12px 34px rgba(21,28,40,.08)}
    pre{overflow:auto;padding:18px;background:#111418;color:#edf2f7;border-radius:8px;line-height:1.6}
    input{width:100%;height:44px;padding:0 12px;border:1px solid #d9dde4;border-radius:6px;font:inherit}
    button,a.button{height:44px;padding:0 16px;border:1px solid #1f6feb;border-radius:6px;background:#1f6feb;color:#fff;font:inherit;font-weight:700;text-decoration:none;cursor:pointer}
    .row{display:grid;grid-template-columns:1fr auto;gap:10px}
    .result{display:none;margin-top:16px;grid-template-columns:1fr auto;gap:12px;align-items:center;padding:12px;border:1px solid #d9dde4;border-radius:8px}
    .result span{overflow-wrap:anywhere;font-family:ui-monospace,Consolas,monospace;font-size:14px}
    @media(max-width:640px){main{width:calc(100% - 24px);padding:32px 0}.row,.result{grid-template-columns:1fr}button,a.button{width:100%}}
  </style>
</head>
<body>
<main>
  <p>Docker Hub 与 GitHub 中转代理</p>
  <h1>ToHub</h1>
  <p>把当前域名配置为 Docker Registry Mirror，并将 GitHub 链接转换为可代理访问的地址。</p>
  <section>
    <h2>Docker daemon.json</h2>
    <pre><code>${dockerConfig}</code></pre>
  </section>
  <section>
    <h2>GitHub 代理访问</h2>
    <p><a class="button" href="/github/" target="_blank" rel="noreferrer">访问首页</a></p>
    <form id="form" class="row">
      <input id="input" placeholder="github.com/trah01 或 trah01/tohub">
      <button>转换</button>
    </form>
    <div id="result" class="result"><span id="url"></span><a id="open" class="button" target="_blank" rel="noreferrer">打开</a></div>
  </section>
</main>
<script>
document.getElementById("form").addEventListener("submit", function(event) {
  event.preventDefault();
  var value = document.getElementById("input").value.trim();
  if (!value) return;
  var path = value;
  if (value === "github.com") {
    path = "/github/";
  } else if (value.indexOf("github.com/") === 0) {
    value = "https://" + value;
  }
  try {
    if (path !== "/github/") {
      var parsed = new URL(value);
      if (parsed.hostname !== "github.com") return;
      path = parsed.pathname + parsed.search + parsed.hash;
    }
  } catch (_) {
    path = value[0] === "/" ? value : "/" + value;
  }
  var converted = location.origin + path;
  document.getElementById("url").textContent = converted;
  document.getElementById("open").href = converted;
  document.getElementById("result").style.display = "grid";
});
</script>
</body>
</html>`;
}
