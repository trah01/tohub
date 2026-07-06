# 设计理念
考虑到网络的因素，在国内无论是github还是dockerhub，全都是无法访问的，所以我想要开发一个用于中转代理的项目，用来代理加速这些hub

# 设计要点
- 支持dockerhub的代理，允许直接配置到docker的daemon.json里，直接作为docker镜像加速
- 可以代理github，不仅仅是可以代理下载，还可以代理访问，比如github.com/trah01，允许拼接为proxy.example.com/trah01,就可以直接访问对应的页面了
- 仅展示daemon.json的配置教程，还有输入github链接，一键访问代理的github地址

# 代码和请求
- 使用go语言开发，使用docker compose进行部署
- 存在一个独立版本，可以使用cloudflare进行部署