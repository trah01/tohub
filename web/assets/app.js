(function () {
  const origin = window.location.origin;
  const config = {
    "registry-mirrors": [origin]
  };

  const dockerConfig = document.getElementById("dockerConfig");
  dockerConfig.textContent = JSON.stringify(config, null, 2);

  document.getElementById("copyDockerConfig").addEventListener("click", async function () {
    await navigator.clipboard.writeText(dockerConfig.textContent);
  });

  document.getElementById("githubForm").addEventListener("submit", function (event) {
    event.preventDefault();
    const input = document.getElementById("githubInput").value.trim();
    const converted = convertGitHubURL(input);
    if (!converted) {
      return;
    }
    document.getElementById("githubProxyURL").textContent = converted;
    document.getElementById("githubOpen").href = converted;
    document.getElementById("githubResult").hidden = false;
  });

  function convertGitHubURL(value) {
    if (!value) {
      return "";
    }
    let path = value;
    if (value === "github.com") {
      return origin + "/github/";
    }
    if (value.startsWith("github.com/")) {
      value = "https://" + value;
    }
    try {
      const parsed = new URL(value);
      if (parsed.hostname !== "github.com") {
        return "";
      }
      path = parsed.pathname + parsed.search + parsed.hash;
    } catch (_) {
      path = value.startsWith("/") ? value : "/" + value;
    }
    return origin + path;
  }
})();
