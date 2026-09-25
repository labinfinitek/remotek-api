# Remotek

[English Doc](README_EN.md)

Remotek 的 API，基于 lejianwen 的 [rustdesk-api](https://github.com/lejianwen/rustdesk-api) v2.7，兼容 RustDesk 客户端，用 Go 实现，包含 Web Admin。


<div align=center>
<img src="https://img.shields.io/badge/golang-1.22-blue"/>
<img src="https://img.shields.io/badge/gin-v1.9.0-lightBlue"/>
<img src="https://img.shields.io/badge/gorm-v1.25.7-green"/>
<img src="https://img.shields.io/badge/swag-v1.16.3-yellow"/>
<img src="https://goreportcard.com/badge/github.com/lejianwen/rustdesk-api/v2"/>
<img src="https://github.com/lejianwen/rustdesk-api/actions/workflows/build.yml/badge.svg"/>
</div>

## 搭配[lejianwen/rustdesk-server]使用更佳。
> [lejianwen/rustdesk-server]fork自RustDesk Server官方仓库
> 1. 解决了使用API链接超时问题
> 2. 可以强制登录后才能发起链接
> 3. 支持客户端websocket



# 特性

- PC端API
    - 个人版API
    - 登录
    - 地址簿
    - 群组
    - 授权登录
      - 支持`github`, `google` 和 `OIDC` 登录，
      - 支持`web后台`授权登录
      - 支持`LDAP`(AD和OpenLDAP已测试), 如果API Server配置了LDAP
    - i18n
- Web Admin
    - 用户管理
    - 设备管理
    - 地址簿管理
    - 标签管理
    - 群组管理
    - Oauth 管理
    - 配置LDAP, 配置文件或者环境变量
    - 登录日志
    - 链接日志
    - 文件传输日志
    - i18n
    - server控制(一些官方的简单的指令 [WIKI](https://github.com/lejianwen/rustdesk-api/wiki/Rustdesk-Command))
- CLI
    - 重置管理员密码

## 功能


### API 服务 
基本实现了PC端基础的接口。支持Personal版本接口，可以通过配置文件`rustdesk.personal`或环境变量`RUSTDESK_API_RUSTDESK_PERSONAL`来控制是否启用

<table>
    <tr>
      <td width="50%" align="center" colspan="2"><b>登录</b></td>
    </tr>
    <tr>
        <td width="50%" align="center" colspan="2"><img src="docs/pc_login.png"></td>
    </tr>
     <tr>
      <td width="50%" align="center"><b>地址簿</b></td>
      <td width="50%" align="center"><b>群组</b></td>
    </tr>
    <tr>
        <td width="50%" align="center"><img src="docs/pc_ab.png"></td>
        <td width="50%" align="center"><img src="docs/pc_gr.png"></td>
    </tr>
</table>

### Web Admin:

* 使用前后端分离，提供用户友好的管理界面，主要用来管理和展示。前端代码在[rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)

* 后台访问地址是`http://<your server>[:port]/_admin/`
* 初次安装管理员用户名为`admin`，初始密码为20位随机字符，只写入`data/admin-password.txt`（与`rustdeskapi.db`同目录，容器中为`/app/data/admin-password.txt`），权限`0600`，不会打印到控制台或日志。请登录后台修改密码并删除该文件，也可以通过[命令行](#CLI)更改密码

1. 管理员界面
   ![web_admin](docs/web_admin.png)
2. 普通用户界面
   ![web_user](docs/web_admin_user.png)

3. 每个用户可以多个地址簿，也可以将地址簿共享给其他用户
4. 分组可以自定义，方便管理，暂时支持两种类型: `共享组` 和 `普通组`
6. Oauth,支持了`Github`, `Google` 以及 `OIDC`, 需要创建一个`OAuth App`，然后配置到后台
    - 对于`Google` 和 `Github`, `Issuer` 和 `Scopes`不需要填写.
    - 对于`OIDC`, `Issuer`是必须的。`Scopes`是可选的，默认为 `openid,profile,email`. 确保可以获取 `sub`,`email` 和`preferred_username`
    - `github oauth app`在`Settings`->`Developer settings`->`OAuth Apps`->`New OAuth App`
      中创建,地址 [https://github.com/settings/developers](https://github.com/settings/developers)
    - `Authorization callback URL`填写`http://<your server[:port]>/api/oidc/callback`
      ，比如`http://127.0.0.1:21114/api/oidc/callback`
7. 登录日志
8. 链接日志
9. 文件传输日志
10. server控制

  - `简易模式`,已经界面化了一些简单的指令，可以直接在后台执行
    ![rustdesk_command_simple](./docs/rustdesk_command_simple.png)

  - `高级模式`,直接在后台执行指令
      * 可以官方指令
      * 可以添加自定义指令
      * 可以执行自定义指令

 
11. **LDAP 支持**, 当在API Server上设置了LDAP(已测试AD和LDAP),可以通过LDAP中的用户信息进行登录 https://github.com/lejianwen/rustdesk-api/issues/114 ,如果LDAP验证失败，返回本地用户

### 自动化文档: 使用 Swag 生成 API 文档，方便开发者理解和使用 API。

1. 后台文档 `<youer server[:port]>/admin/swagger/index.html`
2. PC端文档 `<youer server[:port]>/swagger/index.html`
   ![api_swag](docs/api_swag.png)

### CLI

```bash
# 查看帮助
./apimain -h
```

#### 重置管理员密码
```bash
./apimain reset-admin-pwd <pwd>
```
密码需为15到32个字符（按字符计算，不是字节），`reset-pwd <userId> <pwd>`同样。拒绝密码、用户不存在或更新失败时，命令以非0状态码退出。

## 安装与运行

### 相关配置

* [配置文件](./conf/config.yaml)
* 参考`conf/config.yaml`配置文件，修改相关配置。
* 数据库只支持SQLite（`data/rustdeskapi.db`）：`gorm.type`只能是`sqlite`或为空，其他值（例如旧安装的`mysql`）会让API在启动时以状态码1退出。
* 支持的语言只有`it`和`en`，不设置默认为`it`，为空时是英语。`lang`的其他值（例如旧配置文件的`zh-CN`）会让API在启动时以状态码1退出。请求的`Accept-Language`是其中之一时用它回复（后台发送自己的语言，默认是浏览器的语言），否则用配置的语言：RustDesk客户端不发送该请求头。

### 环境变量
环境变量和配置文件`conf/config.yaml`中的配置一一对应，变量名前缀是`RUSTDESK_API`
下面表格并未全部列出，可以参考`conf/config.yaml`中的配置。

| 变量名                                                    | 说明                                                                             | 示例                           |
|--------------------------------------------------------|--------------------------------------------------------------------------------|------------------------------|
| TZ                                                     | 时区                                                                             | Asia/Shanghai                |
| RUSTDESK_API_LANG                                      | `Accept-Language`未选定语言时使用的语言，只能是`it`或`en`，默认`it`；其他值启动时退出 | `it`,`en`                    |
| RUSTDESK_API_APP_REGISTER                              | 是否开启注册; `true`, `false`  默认`false`                                             | `false`                      |
| RUSTDESK_API_APP_WEB_SSO                               | 是否向客户端提供web后台授权登录(`webauth`); `true`, `false` 默认`false`                         | `false`                      |
| RUSTDESK_API_APP_SHOW_SWAGGER                          | 是否可见swagger文档;`1`显示，`0`不显示，默认`0`不显示                                            | `1`                          |
| RUSTDESK_API_APP_TOKEN_EXPIRE                          | token有效时长                                                                      | `168h`                       |
| RUSTDESK_API_APP_DISABLE_PWD_LOGIN                     | 是否禁用密码登录;  `true`, `false`  默认`false`                                          | `false`                      |
| RUSTDESK_API_APP_REGISTER_STATUS                       | 注册用户默认状态; 1 启用，2 禁用, 默认 1                                                      | `1`                          |
| RUSTDESK_API_APP_CAPTCHA_THRESHOLD                     | 验证码触发次数; -1 不启用， 0 一直启用， >0 登录错误次数后启用 ;默认 `3`                                  | `3`                          |
| RUSTDESK_API_APP_BAN_THRESHOLD                         | 封禁IP触发次数; 0 不启用, >0 同一IP在10分钟内登录错误达到该次数后，该IP的所有请求被拒绝30分钟; 默认 `10` | `10`                         |
| -----BRAND配置-----                                      | ----------                                                                     | ----------                   |
| RUSTDESK_API_BRAND_NAME                                | 产品名称：后台标题、欢迎语中的`{{brand}}`、OAuth登录页面标题；默认`Remotek`                   | `Remotek`                    |
| RUSTDESK_API_BRAND_DIR                                 | `logo.svg`和`favicon.svg`所在目录，通过`/brand/`提供；默认`./resources/brand`             | `./resources/brand`          |
| -----ADMIN配置-----                                      | ----------                                                                     | ----------                   |
| RUSTDESK_API_ADMIN_TITLE                               | 后台标题；为空时等于`brand.name`                                                     | `Remotek`                    |
| RUSTDESK_API_ADMIN_HELLO                               | 后台欢迎语，可以使用`html`                                                               |                              |
| RUSTDESK_API_ADMIN_HELLO_FILE                          | 后台欢迎语文件，如果内容多，使用文件更方便。<br>会覆盖`RUSTDESK_API_ADMIN_HELLO`                        | `./conf/admin/hello.html`    |
| -----GIN配置-----                                        | ----------                                                                     | ----------                   |
| RUSTDESK_API_GIN_TRUST_PROXY                           | 信任的代理IP或CIDR，以`,`分割；默认为空，不信任任何代理，忽略`X-Forwarded-For`和`X-Real-IP`。<br>在反向代理后面必须填写代理的地址，否则验证码和封禁把所有客户端都当作代理的IP | 192.168.1.2,192.168.1.3      |
| -----GORM配置-----                                       | ----------                                                                     | ---------------------------  |
| RUSTDESK_API_GORM_TYPE                                 | 数据库类型，只能是`sqlite`（为空也是sqlite），其他值启动时退出                               | sqlite                       |
| RUSTDESK_API_GORM_MAX_IDLE_CONNS                       | 数据库最大空闲连接数                                                                     | 10                           |
| RUSTDESK_API_GORM_MAX_OPEN_CONNS                       | 数据库最大打开连接数                                                                     | 100                          |
| RUSTDESK_API_RUSTDESK_PERSONAL                         | 是否启用个人版API， 1:启用,0:不启用； 默认启用                                                   | 1                            |
| -----RUSTDESK配置-----                                   | ----------                                                                     | ----------                   |
| RUSTDESK_API_RUSTDESK_ID_SERVER                        | Rustdesk的id服务器地址                                                               | 192.168.1.66:21116           |
| RUSTDESK_API_RUSTDESK_RELAY_SERVER                     | Rustdesk的relay服务器地址                                                            | 192.168.1.66:21117           |
| RUSTDESK_API_RUSTDESK_API_SERVER                       | Rustdesk的api服务器地址                                                              | http://192.168.1.66:21114    |
| RUSTDESK_API_RUSTDESK_KEY                              | Rustdesk的key                                                                   | 123456789                    |
| RUSTDESK_API_RUSTDESK_KEY_FILE                         | Rustdesk存放key的文件                                                               | `./conf/data/id_ed25519.pub` |
| ----PROXY配置-----                                       | ----------                                                                     | ----------                   |
| RUSTDESK_API_PROXY_ENABLE                              | 是否启用代理:`false`, `true`                                                         | `false`                      |
| RUSTDESK_API_PROXY_HOST                                | 代理地址                                                                           | `http://127.0.0.1:1080`      |
| ----JWT配置----                                          | --------                                                                       | --------                     |
| RUSTDESK_API_JWT_KEY                                   | 自定义JWT KEY,为空则不启用JWT<br/>如果没使用`lejianwen/rustdesk-server`中的`MUST_LOGIN`，建议设置为空 |                              |
| RUSTDESK_API_JWT_EXPIRE_DURATION                       | JWT有效时间                                                                        | `168h`                       |


### 运行

#### docker运行

1. 直接docker运行,配置可以通过挂载配置文件`/app/conf/config.yaml`来修改,或者通过环境变量覆盖配置文件中的配置

    ```bash
    docker run -d --name rustdesk-api -p 21114:21114 \
    -v /data/rustdesk/api:/app/data \
    -e TZ=Asia/Shanghai \
    -e RUSTDESK_API_LANG=it \
    -e RUSTDESK_API_RUSTDESK_ID_SERVER=192.168.1.66:21116 \
    -e RUSTDESK_API_RUSTDESK_RELAY_SERVER=192.168.1.66:21117 \
    -e RUSTDESK_API_RUSTDESK_API_SERVER=http://192.168.1.66:21114 \
    -e RUSTDESK_API_RUSTDESK_KEY=<key> \
    lejianwen/rustdesk-api
    ```

2. 使用`docker compose`，参考[WIKI](https://github.com/lejianwen/rustdesk-api/wiki)

#### 构建镜像

`Dockerfile` 从源码构建全部内容，分三个阶段：Go 二进制（静态链接，sqlite 需要
CGO）、固定提交的后台前端 [rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)、
最终的 Alpine 镜像。不需要预先编译的二进制，也不需要 `release/` 目录。

```bash
docker build \
  --build-arg VERSION=<版本> \
  --build-arg REVISION="$(git rev-parse HEAD)" \
  -t remotek-api .
```

- 构建参数：`VERSION` 写入 `resources/version`，由 `/api/version` 返回（默认
  `dev`）；`REVISION` 写入 OCI 标签（默认 `unknown`）；`PANNELLO_COMMIT` 是
  rustdesk-api-web 提交的完整 sha（默认 `3998c2a9213fcd047252776d0f0db33e6717026c`，
  master 的最后一次提交，即 v2.7 打包的版本）；`BRAND_NAME` 是后台的静态标题（默认
  `Remotek`，只允许字母、数字、空格和`. _ -`），见[更换品牌](#更换品牌)。
- 进程以用户 `remotek`（uid/gid 10001）运行，不是 root。`/app/data` 是数据卷
  （数据库、`admin-password.txt`）；`/app/runtime` 存放 `log.txt`（日志同时输出到
  stdout）。
- 挂载的宿主机目录必须对 uid 10001 可写，否则 API 无法启动：
  `sudo chown -R 10001:10001 /data/rustdesk/api`。以 root 运行的镜像写下的数据同样
  需要这样处理。
- `HEALTHCHECK` 访问 `http://127.0.0.1:21114/api/version`：如果修改了
  `RUSTDESK_API_GIN_API_ADDR` 中的端口，请用 `--health-cmd` 覆盖。
- 基础镜像按 digest 固定：更新时标签和 digest 一起修改。

#### 更换品牌

名称和标志集中在一处，更换时不需要修改代码：

- 名称：`brand.name`（`RUSTDESK_API_BRAND_NAME`，默认`Remotek`）。它是后台标题
  （`admin.title`为空时，由`GET /api/admin/config/admin`返回）、欢迎语中的
  `{{brand}}`（与`{{username}}`一样替换，见`conf/admin/hello.html`）以及 OAuth/OIDC
  登录页面的标题。重启 API 即生效。
- 标志和图标：`brand.dir`目录（`RUSTDESK_API_BRAND_DIR`，默认`./resources/brand`）中的
  `logo.svg`和`favicon.svg`，API 通过`/brand/logo.svg`和`/brand/favicon.svg`提供，
  后台从这里读取。替换文件即可，不需要重新构建，例如在容器中挂载：
  `-v /srv/remotek/brand:/app/resources/brand:ro`（文件对 uid 10001 可读）。
  `/brand/`下的文件带有不允许脚本并启用沙箱的`Content-Security-Policy`以及
  `X-Content-Type-Options: nosniff`：SVG 必须自包含（内联样式，图片和字体只能来自
  `/brand/`或`data:`），不能有脚本，也不能引用其他网站的资源。
- 后台在浏览器加载完配置之前显示的静态标题在构建时写入：
  `docker build --build-arg BRAND_NAME=<名称> ...`。`Dockerfile`在`npm run build`之前修改
  固定提交的后台源码中的几行（标志、图标、标题）；如果源码中找不到这些行，构建失败。

不改名的部分：`RUSTDESK_API_`变量前缀、配置中的`rustdesk:`部分、
`/api/admin/rustdesk/*`路由、Go module 路径。

#### 下载release直接运行

[下载地址](https://github.com/lejianwen/rustdesk-api/releases)

#### 源码安装

1. 克隆仓库
   ```bash
   git clone https://github.com/lejianwen/rustdesk-api.git
   cd rustdesk-api
   ```

2. 安装依赖

    ```bash
    go mod tidy
    #安装swag，如果不需要生成文档，可以不安装
    go install github.com/swaggo/swag/cmd/swag@latest
    ```

3. 编译后台前端，前端代码在[rustdesk-api-web](https://github.com/lejianwen/rustdesk-api-web)中
   ```bash
   cd resources
   mkdir -p admin
   git clone https://github.com/lejianwen/rustdesk-api-web
   cd rustdesk-api-web
   npm install
   npm run build
   cp -ar dist/* ../admin/
   ```
4. 运行
    ```bash
    #直接运行
    go run cmd/apimain.go
    #如需先重新生成swagger文档（需要swag）
    go generate -tags tools ./tools
    ```
   > 注意：使用 `go run` 或编译后的二进制时，当前目录下必须存在 `conf` 和 `resources`
   > 目录。如果在其他目录运行，可通过 `-c` 和环境变量
   > `RUSTDESK_API_GIN_RESOURCES_PATH` 指定绝对路径，例如：
   > ```bash
   > RUSTDESK_API_GIN_RESOURCES_PATH=/opt/rustdesk-api/resources ./apimain -c /opt/rustdesk-api/conf/config.yaml
   > ```
5. 编译，如果想自己编译,先cd到项目根目录，然后windows下直接运行`build.bat`,linux下运行`build.sh`,编译后会在`release`
   目录下生成对应的可执行文件。直接运行编译后的可执行文件即可。

6. 打开浏览器访问`http://<your server[:port]>/_admin/`，用户名为`admin`，初始密码在`data/admin-password.txt`中，请及时更改密码并删除该文件。


#### 使用`lejianwen/server-s6`镜像运行

- 已解决链接超时问题
- 可以强制登录后才能发起链接
- github https://github.com/lejianwen/rustdesk-server

```yaml
 networks:
   rustdesk-net:
     external: false
 services:
   rustdesk:
     ports:
       - 21114:21114
       - 21115:21115
       - 21116:21116
       - 21116:21116/udp
       - 21117:21117
       - 21118:21118
       - 21119:21119
     image: lejianwen/rustdesk-server-s6:latest
     environment:
       - RELAY=<relay_server[:port]>
       - ENCRYPTED_ONLY=1
       - MUST_LOGIN=N
       - TZ=Asia/Shanghai
       - RUSTDESK_API_RUSTDESK_ID_SERVER=<id_server[:21116]>
       - RUSTDESK_API_RUSTDESK_RELAY_SERVER=<relay_server[:21117]>
       - RUSTDESK_API_RUSTDESK_API_SERVER=http://<api_server[:21114]>
       - RUSTDESK_API_KEY_FILE=/data/id_ed25519.pub
       - RUSTDESK_API_JWT_KEY=xxxxxx # jwt key
     volumes:
       - /data/rustdesk/server:/data
       - /data/rustdesk/api:/app/data #将数据库挂载
     networks:
       - rustdesk-net
     restart: unless-stopped
       
```


## 其他

- [WIKI](https://github.com/lejianwen/rustdesk-api/wiki)
- [链接超时问题](https://github.com/lejianwen/rustdesk-api/issues/92)
- [修改客户端ID](https://github.com/abdullah-erturk/RustDesk-ID-Changer)


## 鸣谢

感谢所有做过贡献的人!

<a href="https://github.com/lejianwen/rustdesk-api/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=lejianwen/rustdesk-api" />
</a>

## 感谢你的支持！如果这个项目对你有帮助，请点个⭐️鼓励一下，谢谢！

[lejianwen/rustdesk-server]: https://github.com/lejianwen/rustdesk-server