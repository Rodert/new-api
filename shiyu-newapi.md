# shiyu-newapi

## 分支规则

- `shiyu-newapi`：定制开发和部署分支。
- 上游更新需要时再合并到该分支：

```bash
git fetch upstream
git checkout shiyu-newapi
git merge upstream/main
git push origin shiyu-newapi
```

## 镜像

推送 `shiyu-newapi` 后，GitHub Actions 自动构建 `linux/amd64` 和 `linux/arm64` 镜像：

```text
ghcr.io/rodert/new-api:shiyu-newapi
```

首次构建成功后，在 GitHub Packages 将该镜像设为 Public。

## 服务器

服务器只需要 `docker-compose-shiyu.yml`，不需要拉取源码或编译。

部署前，修改其中的默认密码 `123456`：

- PostgreSQL 的 `POSTGRES_PASSWORD` 与 `SQL_DSN` 中密码一致。
- Redis 的 `--requirepass` 与 `REDIS_CONN_STRING` 中密码一致。

数据库和 Redis 不对外暴露端口；域名不是部署必需项。

首次启动或更新：

```bash
docker compose -f docker-compose-shiyu.yml pull new-api
docker compose -f docker-compose-shiyu.yml up -d
```

日常发布：本地提交并推送 `shiyu-newapi`，等待 GitHub Actions 成功后，在服务器执行上面的两条命令。

回滚时，将应用镜像临时改为 Actions 生成的 `sha-<提交短 SHA>` 标签，再执行相同命令。
