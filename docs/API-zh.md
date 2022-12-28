<!-- TOC depthFrom:1 depthTo:6 withLinks:1 updateOnSave:1 orderedList:0 -->

- [Server API](#server-api)
	- [工作原理?](#how-it-works)
	- [一般注意事项](#general-considerations)
	- [连接到服务器](#connecting-to-the-server)
		- [gRPC](#grpc)
		- [WebSocket](#websocket)
		- [长轮询](#long-polling)
		- [带外大文件](#out-of-band-large-files)
		- [在反向代理后面运行](#running-behind-a-reverse-proxy)
	- [用户](#users)
		- [身份验证](#authentication)
			- [创建帐户](#creating-an-account)
			- [登录](#logging-in)
			- [更改验证参数](#changing-authentication-parameters)
			- [重置密码，即“忘记密码”](#resetting-a-password-ie-forgot-password)
		- [暂停用户](#suspending-a-user)
		- [凭证验证](#credential-validation)
		- [访问控制](#access-control)
	- [主题Topics](#topics)
		- [`me` 主题](#me-topic)
		- [`fnd` 和标签: 查找用户和主题](#fnd-and-tags-finding-users-and-topics)
			- [查询语言](#query-language)
			- [查询增量更新](#incremental-updates-to-queries)
			- [查询重写](#query-rewrite)
			- [可能的用例](#possible-use-cases)
		- [点到点主题](#peer-to-peer-topics)
		- [组主题](#group-topics)
		- [`sys` 主题](#sys-topic)
	- [使用服务器发布消息ID](#using-server-issued-message-ids)
	- [用户代理和状态通知](#user-agent-and-presence-notifications)
	- [受信任的公共和私有字段](#trusted-public-and-private-fields)
		- [受信任](#trusted)
		- [公共](#public)
		- [私人](#private)
	- [内容格式](#format-of-content)
	- [带外处理大型文件](#out-of-band-handling-of-large-files)
		- [上传](#uploading)
		- [下载](#downloading)
	- [推送通知](#push-notifications)
		- [Tinode推送网关](#tinode-push-gateway)
		- [Google FCM](#google-fcm)
		- [标准输出](#stdout)
	- [视频通话](#video-calls)
	- [消息](#messages)
		- [客户端到服务器消息](#client-to-server-messages)
			- [`{hi}`](#hi)
			- [`{acc}`](#acc)
			- [`{login}`](#login)
			- [`{sub}`](#sub)
			- [`{leave}`](#leave)
			- [`{pub}`](#pub)
			- [`{get}`](#get)
			- [`{set}`](#set)
			- [`{del}`](#del)
			- [`{note}`](#note)
		- [服务器到客户端消息](#server-to-client-messages)
			- [`{data}`](#data)
			- [`{ctrl}`](#ctrl)
			- [`{meta}`](#meta)
			- [`{pres}`](#pres)
			- [`{info}`](#info)

<!-- /TOC -->

# Server API


## 它是如何工作的？

Tinode 是一个 IM 路由器和商店。从概念上讲，它大致遵循
[发布订阅](https://en.wikipedia.org/wiki/Publish%E2%80%93subscribe_pattern) 模型。

服务器连接会话、用户和主题。会话是客户端应用程序和服务器之间的网络连接。用户表示通过会话连接到服务器的人。主题是一个命名的通信通道，用于在会话之间路由内容。

为用户和主题分配了唯一 ID。用户 ID 是一个前缀为`usr`的字符串，后跟 base64 URL 编码的伪随机 64 位数字，例如`usr2il9suCbuko`。主题 ID 如下所述。

诸如移动或 web 应用程序之类的客户端通过通过 websocket 或通过长轮询连接到服务器来创建会话。要执行大多数操作，需要客户端身份验证。客户端通过发送`{login}`数据包来验证会话。有关详细信息，请参阅[身份验证]（#Authentication）部分。一旦经过身份验证，客户端就会收到一个令牌，稍后用于身份验证。同一用户可以建立多个同时的会话。不支持（也不需要）注销。

一旦会话建立，用户就可以开始通过主题与其他用户交互。以下主题类型可用：

_`me`是用于管理个人资料和接收有关其他主题的通知的主题；每个用户都有`me`的主题。
_`fnd`主题用于查找其他用户和主题；`fnd`主题也适用于每个用户。
*对等话题是两个用户之间严格的交流渠道。每个参与者都将主题名称视为另一个参与者的 ID：'usr'前缀后跟用户 ID 的 base64 URL 编码数字部分，例如`usr2il9suCbuko`。
*组主题是多用户通信的通道。它命名为'grp'，后跟 11 个伪随机字符，即`grpYiqEXb4QY6s`。必须显式创建组主题。

会话通过发送`{sub}`数据包加入主题。数据包`{sub}`有三个功能：创建新主题、向用户订阅主题以及将会话附加到主题。有关详细信息，请参阅下面的[`{sub}`]（#sub）部分。

一旦会话加入主题，用户就可以通过发送`{pub}` 数据包开始生成内容。内容将作为`{data}`数据包传递到其他附加会话。

用户可以通过发送`{get}` 和`{set}`数据包来查询或更新主题元数据。

对主题元数据的更改，如主题描述的更改，或其他用户加入或离开主题时，将使用`{pres}` （状态）数据包向实时会话报告。`{pres}` 数据包被发送到受影响的主题或`me`主题。

当用户的`me`主题联机时（即，经过验证的会话连接到`me`话题），`{pres}`数据包将发送到所有其他用户的`me`主题，这些用户与第一个用户具有对等订阅。

## 一般注意事项

时间戳始终表示为[RFC 3339](http://tools.ietf.org/html/rfc3339)-格式化字符串，精度高达毫秒，时区始终设置为 UTC，例如`"2015-10-06T18:07:29.841Z"`。

每当提到 base64 编码时，它意味着去掉了填充字符的 base64 URL 编码，请参见[RFC 4648](http://tools.ietf.org/html/rfc4648).

`{data}` 数据包具有服务器发出的顺序 ID：以 10 为基数的数字，从 1 开始，每一条消息递增一。保证每个主题都是独一无二的。

为了将请求连接到响应，客户端可以为设置到服务器的所有数据包分配消息 ID。这些 ID 是客户端定义的字符串。客户端应至少在每个会话中使其唯一。客户端分配的 ID 不由服务器解释，而是按原样返回给客户端。

## 连接到服务器

有三种方法可以通过网络访问服务器：websocket、长轮询和[gRPC](https://grpc.io/).

当客户端通过 HTTP(S)（例如通过 websocket 或长轮询）与服务器建立连接时，服务器提供以下端点：

- `/v0/channels` 用于 websocket 连接
- `/v0/channels/lp` 用于长时间轮询
- `/v0/file/u` 用于文件上载
- `/v0/file/s` 用于提供文件（下载）

`v0”表示 API 版本（当前为零）。每个 HTTP(S)请求都必须包含 API 密钥。服务器按以下顺序检查 API 密钥：

- HTTP 标头 `X-Tinode-APIKey`
- URL 查询参数 `apikey` (/v0/file/s/abcdefg.jpeg?apikey=...)
- 表单值 `apikey`
- Cookie `apikey`

为方便起见，每个演示应用程序都包含一个默认的 API 密钥。使用[`keygen`utility]（../keygen）为生产生成自己的密钥。

连接打开后，客户端必须向服务器发出`{hi}`消息。服务器以`{ctrl}`消息响应，该消息指示成功或错误。响应的`params`字段包含服务器的协议版本`params`:{"ver":"0.15"}`，并且可能包含其他值。

### gRPC

请参见[proto 文件]（../pbx/model.proto）中 gRPC API 的定义。gRPC API 的功能比本文中描述的 API 稍多：它允许 `root` 用户代表其他用户发送消息，也可以删除用户。

protobuf 消息中的`bytes`字段需要 JSON 编码的 UTF-8 内容。例如，一个字符串在转换为 UTF-8 字节之前应加引号：`[]byte("\"some string\"")`（Go）， `'"another string"'.encode('utf-8')` （Python 3）。

### WebSocket

消息以文本帧发送，每帧一条消息。二进制帧保留供将来使用。默认情况下，服务器允许使用`Origin`标头中的任何值进行连接。

### 长时间轮询

长轮询通过`HTTP POST`（首选）或`GET`工作。响应客户机的第一个请求，服务器发送一条 `{ctrl}`消息，其中包含`params`中的`sid` （会话 ID）。长轮询客户端必须在 URL 或请求正文中的每个后续请求中包含`sid` 。

服务器允许来自所有源的连接，即：`Access-Control-Allow-Origin: *`

### 带外大型文件

大型文件使用`HTTP POST`作为 `Content-Type: multipart/form-data`在带外发送。有关详细信息，请参见[下文](#out-of-band-handling-of-large-files)。

### 在反向代理后面运行

Tinode 服务器可以设置为在反向代理（如 NGINX）后面运行。为了提高效率，它可以通过将`listen` 和/或 `grpc_listen` 配置参数设置为 Unix 套接字文件的路径，例如`unix:/run/tinode.sock`，来接受来自 Unix 套接字的客户端连接。服务器还可以配置为通过将`use_x_forwarded_for` 配置参数设置为`true`，从`X-Forwarded-For`HTTP 标头读取对等方的 IP 地址。

## 用户

用户意味着代表一个人，一个最终用户：消息的生产者和消费者。

用户通常被指定为两个身份验证级别之一：经过身份验证的`auth`或匿名的`anon`。第三级 `root`只能通过`gRPC`访问，它允许 `root`代表其他用户发送消息。

当首次建立连接时，客户端应用程序可以发送`{acc}`或`{login}`消息，以在一个级别上验证用户。

为每个用户分配一个唯一的 ID。ID 由`usr`组成，后跟 base64 编码的 64 位数值，例如`usr2il9suCbuko`。用户还具有以下属性：

- `created`: 创建用户记录时的时间戳
- `updated`:上次更新用户 `public`或 `trusted`的时间戳
- `status`: 账户状态
- `username`: `basic`身份验证中使用的唯一字符串；其他用户无法访问用户名
- `defacs`: 描述用户与认证用户和匿名用户进行对等对话的默认访问模式的对象；有关详细信息，请参阅[访问控制]（#访问控制）
  - `auth`: 经过身份验证的 `auth`用户的默认访问模式
  - `anon`: 匿名`anon`用户的默认访问权限
- `trusted`: 由系统管理发布的应用程序定义的对象。任何人都可以阅读它，但只有系统管理员可以更改它。
- `public`: 描述用户的应用程序定义对象。任何人都可以向用户查询`public`数据。
- `private`: 应用程序定义的对象，该对象对于当前用户是唯一的并且只能由用户访问。
- `tags`: [发现]（#fnd-and-tags-finding-users-and-topics）和凭据。

用户的帐户具有状态。定义了以下状态：

- `ok` (正常):默认状态，表示该帐户不受任何限制，可以正常使用；
- `susp` (suspended): 用户无法访问帐户，也无法通过[搜索]找到(#fnd-and-tags-finding-users-and-topics)；状态可以由管理员分配并且完全可逆。
- `del` (软删除):用户被标记为已删除，但用户的数据被保留；当前不支持取消删除用户。
- `undef` (未定义):由认证器内部使用；不应在其他地方使用。

differentiate client software.用户可以保持与服务器的多个同时连接（会话）。每个会话都标有客户端提供的 `User Agent`字符串，用于区分客户端软件。

设计不支持注销。如果应用程序需要更改用户，它应该打开一个新的连接，并使用新的用户凭据对其进行身份验证。

### 身份验证

身份验证在概念上类似于[SASL](https://en.wikipedia.org/wiki/Simple_Authentication_and_Security_Layer)：它作为一组适配器提供，每个适配器实现不同的身份验证方法。在帐户注册[`{acc}`]（#acc）期间和[`{login}`]期间（#login）使用身份验证程序。服务器提供了以下开箱即用的身份验证方法：

- `token` 通过加密令牌提供身份验证。
- `basic` 通过登录密码对提供身份验证。
- `anonymous` 是为临时用户设计的，例如通过聊天处理客户支持请求。
- `rest` 是一种[元方法]（../server/auth/rest/），它允许通过 JSON RPC 使用外部身份验证系统。

任何其他身份验证方法都可以使用适配器实现。

`token`旨在成为身份验证的主要手段。令牌的设计方式使令牌身份验证更轻。例如，令牌验证器通常不进行任何数据库调用，所有处理都在内存中完成。所有其他身份验证方法仅用于获取或刷新令牌。获得令牌后，后续登录应使用它。

`basic`身份验证方案要求`secret`是一个 base64 编码的字符串，该字符串由用户名、冒号`:`和计划文本密码组成。`basic`方案中的用户名不能包含冒号字符`:`（ASCII 0x3A）。

`anonymous`方案可用于创建帐户，但不能用于登录：用户使用`anonymous`方式创建帐户，并获得密码令牌，用于后续的`token`登录。如果令牌丢失或过期，用户将无法再访问帐户。

可以使用`logical_names`配置功能更改在验证器中编译的名称。例如，自定义的`rest`身份验证器可以显示为`basic`，而不是默认身份验证器，或者可以对用户隐藏`token`身份验证器。通过在配置文件中提供一组映射来激活该功能： `logical_name:actual_name` 以重命名，或`actual_name:`以隐藏。例如，要将`rest`服务用于基本身份验证，请使用`"logical_names": ["basic:rest"]`。

#### 创建帐户

创建新帐户时，用户必须通知服务器稍后将使用哪种身份验证方法访问该帐户，并在适当时提供共享密钥。创建帐户时只能使用`basic`和`anonymous`。`basic`要求用户生成并向服务器发送唯一的登录名和密码。`anonymous`不交换秘密。

用户可以选择设置 `{acc login=true}`以使用新帐户进行即时身份验证。当`login=false`（或未设置）时，将创建新帐户，但创建帐户的会话的身份验证状态保持不变。当`login=true`时，服务器将尝试使用新帐户验证会话，成功时，对`{acc}` 请求的`{ctrl}`响应将包含验证令牌。这对于“匿名”身份验证尤为重要，因为这是唯一可以检索身份验证令牌的时间。

#### 登录

通过发出 `{login}`请求来执行登录。只能使用`basic`和 `token`登录。对任何登录的响应都是一条`{ctrl}`消息，其中包含代码 200 和令牌，可以在后续登录中使用`token` 身份验证，或代码 300 请求其他信息，例如验证凭据或响应多步骤身份验证中的方法相关质询，或代码 4xx 错误。

令牌具有服务器配置的过期时间，因此需要定期刷新。

#### 更改身份验证参数

用户可以通过发出`{acc}` 请求来更改身份验证参数，例如更改登录名和密码。当前只有`basic`身份验证支持更改参数：

```js
acc: {
  id: "1a2b3", // 字符串，客户端提供的消息id，可选
  user: "usr2il9suCbuko", // 受更改影响的用户，可选
  token: "XMg...g1Gp8+BO0=", // 身份验证令牌（如果会话尚未经过身份验证），可选。
  scheme: "basic", // 正在更新认证方案。
  secret: base64encode("new_username:new_password") // 新参数
}
```

为了只更改密码，`username`应留空，即`secret: base64encode(":new_password")`。

如果会话未通过身份验证，则请求必须包含`token`。它可以是登录期间获得的常规身份验证令牌，也可以是通过[重置密码]（#resetting-a-password）过程接收的受限令牌。如果会话经过身份验证，则不能包含令牌。如果请求被认证为访问级别`ROOT`，则`user`可以被设置为另一个用户的有效 ID。否则它必须为空（默认为当前用户）或等于当前用户的 ID。

#### 重置密码，即“忘记密码”

要重置登录名或密码（或任何其他身份验证机密，如果身份验证程序支持此操作），需要发送 `{login}` 消息，其中`scheme`设置为`reset`，`secret`包含 base64 编码字符串"`authentication scheme to reset secret for`:`reset method`:`reset method value`"。通过电子邮件重置密码的最基本情况是

```js
login: {
  id: "1a2b3",
  scheme: "reset",
  secret: base64encode("basic:email:jdoe@example.com")
}
```

其中`jdoe@example.com`是较早验证的用户电子邮件。

如果电子邮件与注册相匹配，服务器将使用指定的方法和地址发送一条消息，其中包含重置密码的说明。电子邮件包含一个受限制的安全令牌，用户可以将其包含在带有新密码的`{acc}` 请求中，如[更改身份验证参数]（#changing-authentication-parameters）中所述。

### 暂停用户

服务管理员可以挂起用户的帐户。一旦帐户被挂起，用户就不能再登录和使用该服务。

只有`root`用户可以挂起帐户。要挂起帐户，根用户将发送以下消息：

```js
acc: {
  id: "1a2b3", // 字符串，客户端提供的消息id，可选
  user: "usr2il9suCbuko", // 受更改影响的用户
  status: "susp"
}
```

发送带有 `status: "ok"`的相同消息将取消挂起帐户。根用户可以通过对用户的`me` 主题执行`{get what="desc"}`命令来检查帐户状态。

### 凭据验证

服务器可以可选地配置为要求验证与用户帐户和认证方案相关联的某些凭证。例如，可以要求用户提供唯一的电子邮件或电话号码，或者作为帐户注册的一个条件来解决 captcha。

服务器只需更改配置即可支持开箱即用的电子邮件验证。由于需要商业订阅才能发送文本消息（SMS），所以电话号码的验证不起作用。

若需要某些凭据，则用户必须始终将其保持在已验证状态。这意味着如果必须更改所需的凭据，用户必须首先添加并验证新凭据，然后才能删除旧凭据。

在注册时，通过发送`{acc}`消息、使用`{set topic="me"}`添加、使用 `{del topic="me"}`删除以及通过`{get topic="me"}`查询来初始分配凭据。客户端通过发送 `{login}` 或`{acc}`消息来验证凭据。

### 访问控制

访问控制通过访问控制列表（ACL）管理用户对主题的访问。访问权限分别分配给每个用户主题对（订阅）。

访问控制主要用于组主题。它对`me`和 P2P 主题的可用性仅限于管理状态通知和禁止用户发起或继续 P2P 对话。所有频道读取器都具有相同的权限。

用户对主题的访问由两组权限定义：用户所需的权限"want"和主题管理员授予用户的权限"given"。每个权限由位图中的一位表示。它可以存在也可以不存在。实际访问被确定为所需权限和给定权限的位 AND。权限以一组 ASCII 字符的形式在消息中传递，其中存在字符意味着设置权限位：

-无权限：`N`本身不是权限，而是明确清除/未设置权限的指示符。它通常表示不应应用默认权限。

-Join:`J`，订阅主题的权限

-读取： `R`，接收`{data}`数据包的权限

-写：`W`，对主题的`{pub}`权限

-状态：`P`，接收状态更新的权限`{pres}`

-批准：`A`，批准加入主题、删除和禁止成员的请求的权限；具有此权限的用户是主题的管理员

-共享：`S`，邀请其他人加入主题的权限

-删除：`D`，硬删除邮件的权限；只有所有者才能完全删除主题

-所有者：`O`，用户是主题所有者；所有者可以向任何主题成员分配任何其他权限，更改主题描述，删除主题；主题可能只有一个所有者；有些主题没有所有者

当用户订阅某个主题或与另一个用户开始聊天时，访问权限可以显式设置，也可以默认为`defacs`。可以通过发送`{set}`消息来修改访问权限。

客户端可以在`{sub}`和`{set}`消息中设置显式权限。如果权限丢失或设置为空字符串（不是`N`！），Tinode 将使用预先分配的默认权限`defacs`。如果未找到默认权限，则组主题中经过身份验证的用户将获得`JRWPS`访问权限，在 P2P 主题中则获得`JRWPA`；匿名用户将收到`N`（无访问权限），这意味着每个订阅请求都必须得到主题管理器的明确批准。

为两类用户定义了默认访问权限：身份验证和匿名。默认访问值作为"given"权限应用于所有新订阅。主题的默认访问是在主题创建时由 `{sub.desc.defacs}`建立的，所有者随后可以通过发送 `{set}`消息来修改。同样，用户的默认访问权限在帐户创建时由`{acc.desc.defacs}`建立，用户可以通过向`me`主题发送`{set}`消息来修改。

## 话题

主题是一个或多个人的命名沟通渠道。主题具有持久属性。这些主题属性可以通过`{get what="desc"}`消息查询。

独立于进行查询的用户的主题属性：

- `created`：主题创建时间的时间戳
- `updated`：上次更新主题`trusted`, `public`或`private`的时间戳
- `touched`：发送到主题的最后一条消息的时间戳
- `defacs`：描述经过身份验证和匿名用户的主题默认访问模式的对象；有关详细信息，请参阅[访问控制]（#access-control）
- `auth`：已验证用户的默认访问模式
- `anon`：匿名用户的默认访问权限
- `seq`：整数服务器发出的通过主题发送的最新`{data}` 消息的顺序 ID
- `trusted`：由系统管理员发布的应用程序定义的对象。任何人都可以阅读它，但只有管理员可以更改它。
- `public`：描述主题的应用程序定义的对象。任何可以订阅主题的人都可以接收主题的`public`数据。

用户相关主题属性：

- `acs`：描述给定用户当前访问权限的对象；有关详细信息，请参阅[访问控制]（#access-control）
- `want`：此用户请求的访问权限
- `given`：授予此用户的访问权限
- `private`：当前用户唯一的应用程序定义对象。

主题通常有订阅者。其中一个订阅者可以被指定为具有完全访问权限的主题所有者（`O`访问权限）。订阅者列表可以是带有`{get what="sub"}`消息的查询。订阅者列表在`{meta}`消息的`sub`部分中返回。

### `me` 话题

在创建帐户时，为每个用户自动创建主题`me`。它用作管理帐户信息、接收来自用户和感兴趣主题的状态通知的手段。主题`me`没有所有者。无法删除或取消订阅该主题。用户可以`leave`该主题，该主题将停止所有相关的通信，并指示用户处于脱机状态（尽管用户仍可能登录并继续使用其他主题）。

加入或离开`me`将生成`{pres}` 状态更新，发送给具有给定用户和`P`权限集的对等主题的所有用户。

主题`me`是只读的`{pub}`发送给`me`的消息被拒绝。

发送给`me`的消息`{get what="desc"}` 会自动回复一条`{meta}`消息，该消息包含带有主题参数的`desc`部分（请参见[主题]（#主题）部分的介绍）。`me`主题的`public`参数是用户希望向其连接显示的数据。更改它不仅会更改`me`主题的`public`，还会更改显示用户`public`的所有位置，例如所有用户的对等主题的`public`。

发送给`me`的消息`{get what="sub"}`不同于任何其他主题，因为它返回当前用户订阅的主题列表，与预期用户订阅的`me`相反。

- seq：服务器发出的主题中最后一条消息的数字 id
- recv：当前用户自报的 seq 值为 received
- read：当前用户自报的 seq 值为 read
- seen：对于 P2P 订阅，报告用户上次出现的时间戳和用户代理字符串
- when：用户上次联机的时间戳
- ua：上次使用的用户客户端软件的用户代理字符串

发送给`me`的消息`{get what="data"}`被拒绝。

### `fnd` 和标记：查找用户和主题

在创建帐户时，为每个用户自动创建主题 `fnd`。它充当发现其他用户和组主题的端点。用户和组主题可以通过`tags`发现。标签可以在主题或用户创建时选择分配，然后可以通过对`me`或组主题使用 `{set what="tags"}`进行更新。

标记是一个不区分大小写的任意 Unicode 字符串（在服务器上强制为小写），最长 96 个字符，其中可能包含`Letter`和`Number` Unicode 中的字符[类别/类别](https://en.wikipedia.org/wiki/Unicode_character_property#General_Category)以及以下任意 ASCII 字符：`_`, `.`, `+`, `-`, `@`, `#`, `!`, `?`.

标记可以具有用作命名空间的前缀。前缀是一个 2-16 字符的字符串，以字母[a-z]开头，可以包含小写 ASCII 字母和数字，后跟冒号`:`，例如前缀电话标签`tel:+14155551212`或前缀电子邮件标签`email:alice@example.com`. 某些带前缀的标记可选地强制为唯一的。在这种情况下，只有一个用户或主题可以具有这样的标记。某些标签对于用户来说可能是不可变的，即用户添加或删除不可变标签的尝试将被服务器拒绝。

标记在服务器端进行索引，并用于用户和主题发现。搜索返回按匹配标记数降序排序的用户和主题。

为了查找用户或主题，用户将 `fnd`主题的`public`或`private`参数设置为搜索查询（请参见[query language]（#查询语言）），然后发出`{get topic="fnd" what="sub"}`请求。如果同时设置了`public`和`private`，则使用`public`查询。`private`查询在会话和设备之间持久化，即所有用户的会话都看到相同的`private`询问。`public`查询的值是短暂的，即它不会保存到数据库，也不会在用户会话之间共享。`private`查询适用于不经常更改的大型查询，例如在手机上查找用户联系人列表中每个人的匹配项。`public`查询旨在简短而具体，例如查找某个主题或不在联系人列表中的用户。

系统以`{meta}`消息进行响应，其中`sub`部分列出找到的用户的详细信息或格式化为订阅的主题。

主题 `fnd`是只读的`{pub}`发送到 `fnd`的消息被拒绝。

*当前不支持*当新用户使用与给定查询匹配的标记注册时， `fnd`主题将收到新用户的`{pres}`通知。

[Plugins]（../pbx）支持`Find` 服务，可用于将默认搜索替换为自定义搜索。

#### 查询语言

Tinode查询语言用于定义查找用户和主题的搜索查询。查询是一个包含由空格或逗号分隔的原子术语的字符串。单个查询词与用户或主题的标记相匹配。单个术语可以用RTL语言编写，但查询作为一个整体是从左到右解析的。空格被视为`AND`运算符，逗号（以及空格前面和/或后面的逗号）被视为是`OR`运算符。运算符的顺序被忽略：所有`AND`标记被分组在一起，所有`OR`标记被组合在一起`OR`优先于`AND`：如果标记前面跟逗号，则它是`OR`标记，否则是`AND`。例如，`aaa bbb, ccc`(`aaa AND bbb OR ccc`) 被解释为`(bbb OR ccc) AND aaa`。

包含空格的查询项必须将空格转换为下划线` ` -> `_`，例如`new york`->`new_york`。

**一些示例:**

- `flowers`: 查找包含标记“flower”的主题或用户。.
- `flowers travel`: 查找同时包含标记“flowers”和“travel”的主题或用户。
- `flowers, travel`: 查找包含标记“flowers”或“travel”（或两者）的主题或用户。
- `flowers travel, puppies`: 查找包含 `flowers` 和 `travel` 或 `puppies`的主题或用户, 即 `(travel OR puppies) AND flowers`.
- `flowers, travel puppies, kittens`: f查找包含 `flowers`, `travel`, `puppies`, 或 `kittens`之一的主题或用户, 即 `flowers OR travel OR puppies OR kittens`.  `travel` 和 `puppies` 之间的空格被视为 `OR` ,因为 `OR` 优先于 `AND`.

#### 查询的增量更新

_CURRENTLY UNSUPPORTED_ 查询，特别是 `fnd.private` 可以任意大，仅受消息大小限制和基础数据库中查询大小限制的限制。可以增量地添加或删除术语，而不是重写整个查询以添加或删除某个术语。

从左到右处理增量更新请求。它可以多次包含同一术语，即`-a_tag+a_tag` 是有效的请求。

#### 查询重写

通过登录名、电话或电子邮件查找用户需要使用前缀编写查询词，例如`email:alice@example.com` 而不是 `alice@example.com`. 这可能会给最终用户带来问题，因为这需要他们学习查询语言。Tinode通过在服务器上实现 _query rewrite_ 来解决这个问题：如果查询项（tag）不包含前缀，服务器将通过添加适当的前缀来重写它。. 在对 `fnd.public` 的查询中，也保留原始术语 (查询 `alice@example.com` 重写为 `email:alice@example.com OR alice@example.com`), 在对 `fnd.private` 的查询中，只保留重写的项(`alice@example.com` 重写为 `email:alice@example.com`). 例如，所有看起来像电子邮件的术语 `alice@example.com` 被改写为 `email:alice@example.com OR alice@example.com`. 看起来像电话号码的术语被转换为 [E.164](https://en.wikipedia.org/wiki/E.164) 并且也重写为 `tel:+14155551212 OR +14155551212`. 此外，在对 `fnd.public` 的查询中，所有其他看起来像登录名的非固定术语都会重写为登录名: `alice` -> `basic:alice OR alice`.

如上所述，看起来像电话号码的标签被转换为 E.164 格式。这种转换需要 ISO 3166-1 alpha-2 国家代码。将电话号码标签转换为 E.164时使用以下逻辑:

- 如果标签已经包含国家呼叫代码，则按如下方式使用: `+1(415)555-1212` -> `+14155551212`.
- 如果标记没有前缀，则国家/地区代码取自客户端在 `{hi}` 消息的 `lang` 字段中设置的区域设置值。
- 如果客户端未在 `hi.lang`中提供代码，则国家代码取自`default_country_code` 的`tinode.conf`字段.
- 如果在  `tinode.conf` 中未设置 `default_country_code`, 则使用`US` 国家代码

#### 可能的使用案例
* 将用户限制在组织内。
  可以为用户分配一个不可变的标记，该标记表示用户所属的组织。当用户搜索其他用户或主题时，可以限制搜索始终包含该标记。这种方法可用于将用户划分为彼此可见性有限的组织。

* 按地理位置搜索。
  客户端软件可以定期分配 [geohash](https://en.wikipedia.org/wiki/Geohash) 基于当前位置向用户标记。搜索给定区域中的用户意味着匹配geohash标签。

* 按数字范围搜索，如年龄范围。
  该方法类似于geohashing。整个数字范围由2的最小可能幂覆盖，例如，人类年龄范围由2<sup>7</sup>=128 岁覆盖。整个范围被分成两半：0-63范围由0表示，64-127范围由1表示。 对每个子范围重复操作，即0-31为00，32-63为01，0-15为000，32-47为010。一旦完成，30岁将属于以下范围: 0 (0-63), 00 (0-31), 001 (16-31), 0011 (24-31), 00111 (28-31), 001111 (30-31), 0011110 (30). A 30 y.o. 一个30岁的用户被分配了几个标签来指示年龄，即`age:00111`, `age:001111`,  和 `age:0011110`. 从技术上讲，可以分配所有7个标签，但通常这是不切实际的。要查询年龄范围28-35中的任何人，请将该范围转换为最小数量的标签: `age:00111` (28-31), `age:01000` (32-35). 此查询将通过标记 `age:00111`匹配30岁的用户.


### 点到点主题

点到点 (P2P) 主题代表严格意义上两个用户之间的通信渠道。两名参与者的主题名称各不相同。他们每个人都将主题的名称视为另一个参与者的用户ID: `usr` 后跟用户的base64 URL编码ID. 例如，如果两个用户`usrOj0B3-gSBSs` 和 `usrIU_LOVwRNsc` 启动P2P主题，第一个用户将其视为 `usrIU_LOVwRNsc`, 第二个用户将视为 `usrOj0B3-gSBSs`. P2P主题没有所有者。

P2P主题由一个用户创建，该用户订阅的主题的名称等于另一个用户的ID。例如，用户 `usrOj0B3-gSBSs` 可以通过发送 `{sub topic="usrIU_LOVwRNsc"}`与用户`usrIU_LOVwRNsc`建立P2P主题。  Tinode将响应一个`{ctrl}` 包，其中包含如上所述的新创建主题的名称。另一个用户将收到关于`me` 主题的`{pres}` m消息，并具有更新的访问权限。

P2P主题的 'public' 参数取决于用户。例如，用户A和B之间的P2P主题将向用户B显示用户A的'public' ，反之亦然。如果用户更新了 'public'，则所有用户的P2P主题也会自动更新 'public'.

P2P主题的 'private' 参数与任何其他主题类型一样，由每个参与者单独定义。

### 分组主题

组主题表示多个用户之间的通信通道。组主题的名称是 `grp` 或 `chn` ，后跟base64 URL编码集的字符串。不能对组名称的内部结构或长度进行其他假设。

组主题支持有限数量的订阅者(由配置文件中的 `max_subscriber_count` 参数控制) 每个订阅者的访问权限都单独管理。还可以启用组主题以支持任意数量的只读用户 - `readers`. 所有 `readers` 都具有相同的访问权限。启用 `readers` 的组主题称为 `channels`.

通过发送`{sub}` 消息来创建组主题，其中主题字段设置为字符串 `new` 或 `nch` 可选后跟任何字符，例如 `new` 和 `newAbC123` 等同。Tinode将以 `{ctrl}` 消息响应，消息中包含新创建的主题的名称，即 `{sub topic="new"}` 将以 `{ctrl topic="grpmiKBkQVXnm3P"}`.回复。如果主题创建失败，则在原始主题名称上报告错误，即 `new` or `newAbC123`. 创建主题的用户成为主题所有者。所有权可以通过 `{set}` 消息转移给另一个用户，但一个用户必须始终是所有者。

`channel` 主题在以下方面与非频道组主题不同:

 * 频道主题是通过发送 `{sub topic="nch"}`创建的。发送 `{sub topic="new"}` 将创建组主题，而不启用频道功能。
 * 发送 `{sub topic="chnAbC123"}` 将创建对频道的 `reader` 订阅。非频道主题将拒绝此类订阅请求。
 * 使用 [`fnd`](#fnd-and-tags-finding-users-and-topics)搜索主题时，频道将显示带有 `chn` 前缀的地址，非频道主题将显示带有 `grp` 前缀.
 * 接收人在频道上收到的消息没有 `From` 字段。普通订户将收到包含发件人ID的 `From` 消息。
 * 频道和非频道组主题的默认权限不同：频道组主题根本不授予任何权限。
 * 加入或离开主题（常规或启用频道）的订户将向当前处于主题加入状态并具有适当权限的所有其他订户生成`{pres}` 消息。读取器加入或离开通道不会生成 `{pres}` 消息。

### `sys` 主题

`sys` 主题是与系统管理员始终可用的沟通渠道。普通非根用户不能订阅 `sys` 但可以在不订阅的情况下发布到它。现有客户机使用此通道通过发送草稿格式的 `{pub}` 消息并将报告作为JSON附件来报告滥用。根用户可以订阅 `sys` 主题。订阅后，根用户将收到其他用户发送到 `sys` 主题的消息。

## 使用服务器颁发的消息ID

Tinode以服务器发出的顺序消息ID的形式为客户端缓存 `{data}` 消息提供基本支持。客户端可以通过发出 `{get what="desc"}` 消息来请求主题的最后一个消息id。如果返回的ID大于最近收到的消息的ID，则客户端知道该主题包含未读消息及其计数。客户端可以使用 `{get what="data"}` 消息获取这些消息。客户端还可以使用消息ID对历史检索进行分页。

## 用户代理和状态通知

当一个或多个用户会话附加到 `me` 主题时，报告用户处于联机状态。 客户端软件使用 `{login}` `ua` (用户代理) 字段向服务器标识自己. _用户代理_ 以下方式发布在 `{meta}` 和 `{pres}` 消息中:

 * 当用户的第一个会话连接到 `me`时, 该会话的 _用户代理_ 将在 `{pres what="on" ua="..."}` 消息中广播.
 * 当多个用户会话附加到 `me`时, 最近发生操作的会话的 _用户代理_  将以`{pres what="ua" ua="..."}`报告；在此上下文中的 'action' 表示客户端发送的任何消息。为了避免潜在的过多流量，用户代理更改的广播频率不超过每分钟一次。
 * 当用户的最后一个会话与`me`分离时, 该会话的 _用户代理_ 将与时间戳一起记录；用户代理在 `{pres what="off"  ua="..."}` 消息中广播，随后报告为最后一个在线时间戳和用户代理。

未报告空的 `ua=""` _用户代理_ 例如，如果用户使用非空用户代理连接到 `me` 然后使用空代理连接，则不会报告更改。将来可能不允许使用空的用户代理。

## 可信、公共和私有字段(Trusted, Public, 和 Private 字段)

主题和订阅具有 `trusted`, `public`, 和 `private` f字段。通常，字段由应用程序定义。除了 `fnd` 主题之外，服务器不强制这些字段的任何特定结构。同时，出于互操作性的原因，客户端软件应使用相同的格式。以下各节描述了所有官方客户实现的这些字段的格式。

### 受信任的(Trusted)

组和对等主题中可选的 `trusted` 字段的格式是一组键值对; `fnd` 和 `sys` 主题没有 `trusted`. 当前定义了以下可选键:
```js
trusted: {
  verified: true, // 布尔值，已验证/可信用户或主题的指示符.
  staff: true,    // 布尔值，表示用户或主题
                  // 是服务器管理的一部分/属于服务器管理。
  danger: true    // 布尔值，表示用户或主题不可信。
}
```

### 公共(Public)

组、对等、系统主题中`public` 字段的格式应为 [theCard](./thecard.md).

`fnd` 主题要求 `public` 是表示 [搜索查询](#query-language)的字符串.

### 私有(Private)

组和对等主题中 `private` 字段的格式是一组键值对。当前定义了以下键:
```js
private: {
  comment: "some comment", // string, 关于主题或对等用户的可选用户注释
  arch: true, // boolean, 表示主题由用户存档，即
              // 不应与其他非归档主题一起显示在UI中.
  starred: false,  // boolean, 表示主题由用户加星号或固定.
  accepted: "JRWS" // string, 用户接受的'given' 模式.
}
```

虽然尚未强制执行，但自定义字段应该以 `x-` 开头，后跟应用程序名称，例如 `x-myapp-value: "abc"`. 字段应仅包含基本类型，即 `string`, `boolean`, `number`, 或 `null`.

`fnd` 主题要求 `private` 是表示 [搜索查询](#query-language)的字符串。

## 内容的格式

`{pub}` 和 `{data}`中的 `content` 字段的格式是应用程序定义的，因此服务器不强制执行字段的任何特定结构。同时，出于互操作性的原因，客户端软件应使用相同的格式。目前支持以下两种类型的 `content` :
 * 纯文本
 * [草稿](./drafty.md)

如果使用草稿，则必须设置消息标题 `"head": {"mime": "text/x-drafty"}`.


## 大型文件的带外处理

由于多种原因，大文件在带内发送时会产生问题:
 * 带内消息存储在数据库字段中时对数据库存储的限制
 * 作为下载聊天记录的一部分，必须完全下载带内消息

Tinode 为处理大型文件提供了两个端点: `/v0/file/u` 用于上载文件 `v0/file/s` 用于下载。端点要求客户端同时提供 [API key](#connecting-to-the-server) 和登录凭据。服务器按以下顺序检查凭据:

**登录凭据**
 * HTTP 标头 `Authorization` (https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Authorization)
 * URL 查询参数 `auth` 和 `secret` (/v0/file/s/abcdefg.jpeg?auth=...&secret=...)
 * 表单值 `auth` 和 `secret`
 * Cookies `auth` 和 `secret`

### 正在上载

要上载文件，首先创建 RFC 2388 多部分请求，然后使用 HTTP POST. 将其发送到服务器。服务器用`307 Temporary Redirect` 和新上传URL或`200 OK` 和 `{ctrl}` 消息来响应请求:

```js
ctrl: {
  params: {
    url: "/v0/file/s/mfHLxDWFhfU.pdf"
  },
  code: 200,
  text: "ok",
  ts: "2018-07-06T18:47:51.265Z"
}
```
如果返回 `307 Temporary Redirect` ，则客户端必须在提供的URL处重试上载。 `307` 响应中返回的URL应仅用于这一次上载。所有后续上传都应首先尝试默认URL。

`ctrl.params.url` 包含当前服务器上载文件的路径。它可以是完整路径，如 `/v0/file/s/mfHLxDWFhfU.pdf`, 也可以是相对路径，如 `./mfHLxDWFhfU.pdf`, 或仅文件名 `mfHLxDWFhfU.pdf`. 除了完整路径之外的任何内容都将根据默认的 *download* 端点 `/v0/file/s/`. 进行解释。例如，如果返回`mfHLxDWFhfU.pdf` 则该文件位于 `http(s)://current-tinode-server/v0/file/s/mfHLxDWFhfU.pdf`.

一旦接收到文件的URL，无论是立即还是在重定向之后，客户端都可以使用该URL发送 `{pub}` 息，并将上传的文件作为附件，或者如果文件是图像，则作为主题或用户配置文件的化身图像(请参见[theCard](./thecard.md)). 例如，URL可以用于[Drafty](./drafty.md)-格式的 `pub.content` 字段:

```js
{
  pub: {
    id: "121103",
    topic: "grpnG99YhENiQU",
    head: {
      mime: "text/x-drafty"
    },
    content: {
      ent: [
      {
        data: {
        mime: "image/jpeg",
        name: "roses-are-red.jpg",
        ref:  "/v0/file/s/sJOD_tZDPz0.jpg",
        size: 437265
      },
        tp: "EX"
      }
    ],
    fmt: [
      {
        at: -1,
      key:0,
      len:1
      }
    ]
    }
  },
  extra: {
    attachments: ["/v0/file/s/sJOD_tZDPz0.jpg"]
  }
}
```

在 `extra: attachments[...]` 字段中列出使用的URL很重要。 Tinode 服务器使用此字段维护上传文件的使用计数器。一旦给定文件的计数器降至零 (例如，因为删除了具有共享URL的消息，或者因为客户端未能将URL包含在 `extra.attachments` 字段中), 服务器将垃圾收集该文件。只能使用相对URL。忽略`extra.attachments` 字段中的绝对URL。URL值应为响应上载而返回的 `ctrl.params.url`.

### 正在下载

服务端点 `/v0/file/s` s响应HTTP GET请求提供文件。客户端必须针对此端点评估相对URL，即，如果它接收到URL `mfHLxDWFhfU.pdf` 或 `./mfHLxDWFhfU.pdf` 它应该将其解释为当前Tinode HTTP服务器上的路径`/v0/file/s/mfHLxDWFhfU.pdf` .

_重要！_ 作为一种安全措施，如果下载URL是绝对的并且指向另一个服务器，则客户端不应发送安全凭据。

## 推送通知

Tinode使用编译时适配器来处理推送通知。服务器附带 [Tinode 推送网关](../server/push/tnpg/), [Google FCM](https://firebase.google.com/docs/cloud-messaging/), 和 `stdout` 适配器. Tinode 推送网关 and Google FCM 支持 Android 和 [播放服务](https://developers.google.com/android/guides/overview) (某些中国手机可能不支持), iOS 设备和除 Safari以外的所有主要网络浏览器. `stdout` 适配器实际上不发送推送通知。它主要用于调试、测试和日志记录。其他类型的推送通知，如[TPNS](https://intl.cloud.tencent.com/product/tpns) 可以通过编写适当的适配器来处理。

如果您正在编写自定义插件，则通知负载如下：
```js
{
  topic: "grpnG99YhENiQU", // 接收消息的主题。
  xfrom: "usr2il9suCbuko", // 发送消息的用户的ID。
  ts: "2019-01-06T18:07:30.038Z", // /RFC3339格式的消息时间戳。
  seq: "1234", // 消息的顺序ID（以文本形式发送的整数值）。
  mime: "text/x-drafty", // 可选消息mime类型。
  content: "Lorem ipsum dolor sit amet, consectetur adipisci", // 消息内容的前80个字符为纯文本。
}
```

### Tinode 推送网关

Tinode 推送网关(TNPG) 是一种专有的Tinode服务，代表Tinode发送推送通知。在内部，它使用谷歌FCM，因此支持与FCM相同的平台。与FCM相比，使用TNPG的主要优点是配置简单：移动客户端不需要重新编译，只需要在服务器上进行 [配置更新](../server/push/tnpg/).

### Google FCM

[Google FCM](https://firebase.google.com/docs/cloud-messaging/) 通过[播放服务](https://developers.google.com/android/guides/overview), iPhone 和 iPad devices, 设备，以及除Safari之外的所有主要网络浏览器. 为了使用FCM移动客户端（iOS、Android），必须使用从谷歌获得的凭据重新编译。有关详细信息，请参阅[说明](../server/push/fcm/) for details.

### 标准输出

`stdout` 适配器对于调试和日志记录非常有用。它将推送负载写入 `STDOUT` 在那里它可以被重定向到文件或由其他进程读取。

## 视频通话

[请参见单独的文档](call-establishment.md).

## 消息

消息是一组逻辑关联的数据。消息以JSON格式的UTF-8文本传递。

所有客户端到服务器消息都可能有一个可选的 `id` 字段。客户端将其设置为从服务器接收消息已接收和处理的确认的方式。 `id` 应为会话唯一字符串，但可以是任何字符串。除了检查JSON的有效性之外，服务器不会尝试解释它。当服务器回复客户端消息时，`id` 将返回不变。

服务器要求严格有效的JSON，包括字段名周围的双引号。为了简洁起见，下面的符号省略了字段名称周围的双引号以及外部大括号。示例使用`//` 注释仅用于表达。注释不能用于与服务器的实际通信。



对于更新应用程序定义数据的消息，如`{set}` `private` 或 `public` 字段，当服务器端

需要清除数据，请使用带有单个Unicode DEL字符 "&#x2421;" (`\u2421`)的字符串. 例如 发送 `"public": null` 不会清除字段，但发送 `"public": "␡"` 会的.

服务器会自动忽略任何无法识别的字段。

### 客户端到服务器消息

每个客户端到服务器消息都包含以下部分中描述的主要负载和可选的顶级字段`extra`:
```js
{
  abc: { ... }, // 主有效负载，请参阅以下部分。
  extra: {
    attachments: ["/v0/file/s/sJOD_tZDPz0.jpg"], // 必须免除GC的带外附件数组。
    obo: "usr2il9suCbuko", // 根用户设置的备用用户ID（obo=代表）。
    authlevel: "auth"  // 更改了根用户设置的身份验证级别。
  }
}
```
`attachments` 数组列出了带外上传文件的URL。这样的列表增量使用这些文件的计数器。一旦使用计数器降至0，文件将自动删除。
`obo` 可由 `root` 用户设置。如果设置了`obo` ，服务器会将消息视为来自特定用户，而不是实际发件人。
The `authlevel` 是对 `obo` 允许为用户设置自定义身份验证级别。如果字段未设置，则使用 `"auth"` 级别。

#### `{hi}`

握手消息客户端用于通知服务器其版本和用户代理。此消息必须是第一条

客户端向服务器发送。服务器以`{ctrl}` 响应，其中包含服务器内部版本 `build`, 有线协议版本 `ver`,
在长轮询的情况下，会话ID `sid` 以及服务器约束，都在 `ctrl.params`中.

```js
hi: {
  id: "1a2b3",     // string, 客户端提供的消息id，可选
  ver: "0.15.8-rc2", // string, 客户端支持的有线协议版本，必填
  ua: "JS/1.0 (Windows 10)", // string, 标识客户端软件的用户代理，可选的
  dev: "L1iC2...dNtk2", // string, 标识此特定值的唯一值
                   // 用于推送通知的连接设备；不由服务器解释。
                   // 请参阅[推送通知支持](#push-notifications-support); 可选择的
  platf: "android", // string, 用于推送通知的底层操作系统
                    // "android", "ios", "web"; 如果丢失，服务器将尽力
                    //从所述用户代理串检测所述平台；可选择的
  lang: "en-US"    // 客户端设备的人类语言；可选择的
}
```
用户代理 `ua` 应遵循 [RFC 7231 section 5.5.3](http://tools.ietf.org/html/rfc7231#section-5.5.3) 建议，但格式未强制执行。可以多次发送消息以更新 `ua`, `dev` 和 `lang` 值。如果发送了多次，则第二条和后续消息的 `ver` 字段必须保持不变或未设置。

#### `{acc}`

消息 `{acc}` 创建用户或更新现有用户的 `tags` 或身份验证凭据 `scheme` 和`secret` 要创建一个新的用户，请将 `user` 设置为字符串 `new` 可选地后跟任何字符序列，例如 `newr15gsr`. 身份验证或匿名会话都可以发送 `{acc}` 消息以创建新用户。要更新身份验证数据或验证当前用户的凭据，请取消设置 `user`.

The `{acc}` 消息 **不能** 用于修改现有用户的 `desc` 或`cred` 改为更新用户的`me` 主题.

```js
acc: {
  id: "1a2b3", // string, c客户端提供的消息id，可选
  user: "newABC123", // string, "new" 可选后跟任意字符以创建新用户，
              // 默认值：当前用户，可选
  token: "XMgS...8+BO0=", // string, 用于请求的身份验证令牌,如果
                          //会话未经过身份验证，可选
  status: "ok", // 更改用户状态；无默认值，可选。
  scheme: "basic", // a此帐户的身份验证方案，必填；
                   //帐户创建当前支持 "basic" 和 "anon" .
  secret: base64encode("username:password"), // string, 所选的base64编码密码
              // 认证方案；要删除方案，请使用带有单个DEL的字符串
              //Unicode字符 "\u2421"; "token" 和 "basic" 无法删除
  login: true, // boolean, 使用新创建的帐户验证当前会话，
               //即创建帐户并立即使用它登录。
  tags: ["alice johnson",... ], // a用于用户发现的标签数组；请参见 'fnd' 主题
              //详细信息，可选（如果缺少，则除了
              //通过登录）
  cred: [  // 需要验证的帐户凭据，如电子邮件或电话号码。
    {
      meth: "email", // string, 验证方法，例如 "email", "tel", "recaptcha"等.
      val: "alice@example.com", // string, 要验证的凭据，如电子邮件或电话
      resp: "178307", // string, 验证响应，可选
      params: { ... } // 参数，特定于验证方法，可选
    },
  ...
  ],

  desc: {  // 用户初始化数据与表初始化数据紧密匹配；仅在创建帐户时使用；可选
    defacs: {
      auth: "JRWS", // string, 点对点会话的默认访问模式
                    //此用户和其他经过身份验证的用户之间
      anon: "N"  // string, 点对点对话的默认访问模式
                 //在此用户和匿名（未经身份验证）用户之间
    }, // 用户点对点主题的默认访问模式
    public: { ... }, // 用于描述用户的应用程序定义的负载，
                  //每个人都可以使用
    private: { ... } // 专用应用程序定义的负载仅对用户可用
                     //通过 'me' 主题
  }
}
```

服务器以 `{ctrl}` 消息响应，消息中包含 `params` 其中包含新用户帐户的详细信息，例如用户ID，如果是 `login: true`, 则包含身份验证令牌。如果缺少 `desc.defacs` 服务器将为新帐户分配服务器默认访问权限。

用于创建帐户的唯一支持的身份验证方案是`basic` 和 `anonymous`.

#### `{login}`

登录用于验证当前会话。

```js
login: {
  id: "1a2b3",     // string, 客户端提供的消息id，可选
  scheme: "basic", // string, 身份验证方案; "basic",
                   // "token", and "reset" are currently supported
  secret: base64encode("username:password"), // string, 所选的base64编码密码
                    //身份验证方案，必需
  cred: [
    {
      meth: "email", // string, 验证方法，例如 "email", "tel", "captcha"等，必填
      resp: "178307" // string, 验证响应，必填
    },
  ...
  ],   // 响应凭证验证请求，可选
}
```

服务器用  `{ctrl}`消息响应`{login}` 数据包。消息的`params` o包含作为`user`. `token` 包含可用于身份验证的加密字符串。令牌的过期时间作为 `expires`传递。

#### `{sub}`

`{sub}` 数据包具有以下功能:
 * 创建新主题
 * 向用户订阅现有主题
 * 将会话附加到先前订阅的主题
 * 获取主题数据

用户通过发送 `{sub}` 包创建新的组主题，其中 `topic` 字段设置为 `new12321` （常规主题）或`nch12321` (channel) 其中 `12321` 表示包含空字符串的任何字符串。服务器将创建一个主题，并使用新创建的主题的名称返回会话。

用户通过发送 `{sub}` 包创建新的对等主题，其中 `topic` 设置为对等用户的用户ID。

用户始终订阅，会话附加到新创建的主题。

如果用户与主题没有关系，则发送`{sub}` 数据包将创建它。订阅意味着在会话的用户与主题之间建立一种过去不存在关系的关系。

加入（附加到）主题意味着会话开始使用主题中的内容。服务器会根据上下文自动区分订阅和加入/附加：如果用户之前与主题没有任何关系，服务器会订阅用户，然后将当前会话附加到主题。如果存在关系，则服务器仅将会话附加到主题。订阅时，服务器根据主题的访问控制列表检查用户的访问权限。它可能会授予立即访问权，拒绝访问权，可能会生成主题经理的批准请求。


服务器使用 `{ctrl}`回复`{sub}`.

`{sub}` 消息可以包括  `get` 和 `set` 字段，它们镜像 `{get}` 和 `{set}` 信息。如果包含，服务器会将它们视为同一主题的后续 `{set}` 和 `{get}` 消息。如果设置了`get` ，则回复可能包含 `{meta}` 和 `{data}` 消息.


```js
sub: {
  id: "1a2b3",  // string, 客户端提供的消息id，可选
  topic: "me",  // 要订阅或附加的主题
  bkg: true,    // 附加到主题的请求是由自动代理发出的，
                // 服务器应该延迟发送状态通知，
                // 因为代理预计会很快断开主题初始化数据的对象，
                // 仅新主题和新订阅，镜像｛set｝消息
  set: {
  // 新主题参数，镜像{set desc}
    desc: {
      defacs: {
        auth: "JRWS", // string, 新认证订户的默认访问权限
        anon: "N"    // string, 新匿名（未经身份验证）订阅者的默认访问权限
      }, // 新主题的默认访问模式
      trusted: { ... }, // 由系统管理分配的应用程序定义的负载
      public: { ... }, // 用于描述主题的应用程序定义的负载
      private: { ... } // 每个用户的专用应用程序定义的内容
    }, // object，可选

    // 订阅参数，镜像｛set sub｝. 'sub.user' 必须为空
    sub: {
      mode: "JRWS", // string, 请求的访问模式，可选；
                   // 默认值：服务器定义
    }, // object, 可选

    tags: [ // 字符串数组, 更新tags（参见fnd主题描述），可选。
        "email:alice@example.com", "tel:1234567890"
    ],

    cred: { // 更新凭据，可选。
      meth: "email", // string, 验证方法，例如 "email", "tel", "recaptcha"等.
      val: "alice@example.com", // string, 要验证的凭据，如电子邮件或电话
      resp: "178307", // string, 验证响应，可选
      params: { ... } // 参数，特定于验证方法，可选
    }
  },

  get: {
    // 要从主题请求的元数据；空格分隔列表，有效字符串为
    //  "desc", "sub", "data", "tags"; 默认值：不请求任何内容；忽略未知字符串；
    // 有关详细信息，请参阅 {get  what} 
    what: "desc sub data", // string, 可选

    // {get what="desc"} 的可选参数
    desc: {
      ims: "2015-10-06T18:07:30.038Z" // timestamp, ，“如果修改自”
      // -仅当在指定的时间戳之后至少更新了其中一个值时，
      // 才返回公共值和私有值，可选
    },

    //  {get what="sub"} 的可选参数
    sub: {
      ims: "2015-10-06T18:07:30.038Z", // timestamp, "如果修改自" - 仅返回在指定时间戳之后修改的订阅，可选
      user: "usr2il9suCbuko", // string, 返回单个用户的结果，
                              //除'me'以外的任何主题，可选
      topic: "usr2il9suCbuko", // string, 返回单个主题的结果，
                            // 仅限'me' 主题，可选
      limit: 20 // integer, 限制返回对象的数量
    },

    // {get what="data"} 的可选参数，有关详细信息，请参阅 {get what="data"}
    data: {
      since: 123, // integer, 加载服务器发出的ID大于或等于此（包含/关闭）的消息，可选
      before: 321, // integer, 加载服务器发出的序列ID小于此的消息（独占/打开），可选
      limit: 20, // integer, 限制返回对象的数量，
                 //默认值：32，可选
    } // object, 可选
  }
}
```

有关`private`和`public`格式的注意事项，请参见 [公共和私有字段](#public-and-private-fields) .

#### `{leave}`

这是`{sub}` 消息的对应项。它还具有两个功能：
* 离开主题而不取消订阅 (`unsub=false`)
* 取消订阅 (`unsub=true`)

服务器用 `{ctrl}`数据包响应  `{leave}` . 离开而不取消订阅仅影响当前会话。离开并取消订阅将影响所有用户的会话。

```js
leave: {
  id: "1a2b3",  // string, 客户端提供的消息id，可选
  topic: "grp1XUtEhjv6HND",   // string, 要离开的主题，取消订阅，或
               //删除，必填
  unsub: true // boolean, 离开并取消订阅, 可选，默认值：false
}
```

#### `{pub}`

该消息用于向主题订阅者分发内容。

```js
pub: {
  id: "1a2b3", // string, 客户端提供的消息id，可选
  topic: "grp1XUtEhjv6HND", // string, 要发布到的主题，必需
  noecho: false, // boolean, 抑制回声（见下文），可选
  head: { key: "value", ... }, // 一组字符串键值对，传递给｛data｝时未更改，可选
  content: { ... }  // object, 要发布到主题订阅者的应用程序定义内容，必需
}
```

主题订阅者接收 [`{data}`](#data) 消息中的 `content` 。默认情况下，发起会话会像当前附加到主题的任何其他会话一样获得`{data}` 的副本。如果由于某种原因，发起会话不想接收它刚刚发布的数据副本，请将`noecho` 设置为 `true`.

有关`content`格式的注意事项，请参见[内容格式](#format-of-content) .

当前为`head`字段定义了以下值：

 * `attachments`: 表示附加到此邮件的媒体的路径数组 `["/v0/file/s/sJOD_tZDPz0.jpg"]`.
 * `auto`: `true` 当消息自动发送时，即通过聊天机器人或自动应答器发送。
 * `forwarded`: 消息是转发消息的指示符，原始消息的唯一ID，`"grp1XUtEhjv6HND:123"`.
 * `hashtags`: 消息中没有前导`#`符号的哈希标签数组： `["onehash", "twohash"]`.
 * `mentions`: 消息中提到的用户ID数组 (`@alice`) : `["usr1XUtEhjv6HND", "usr2il9suCbuko"]`.
 * `mime`: 消息内容的mime类型, `"text/x-drafty"`; `null` 或缺少值被解释为 `"text/plain"`.
 * `priority`: 消息显示优先级：提示客户端消息应在一段时间内更突出地显示；当前仅定义了`"high"`; `{"level": "high", "expires": "2019-10-06T18:07:30.038Z"}`; `priority` 只能由主题所有者或管理员设置 (`A` 权限). `"expires"` 限定符是可选的。
 * `replace`: 消息是另一消息的更正/替换的指示符，正在更新/替换的消息的主题唯一ID，`":123"`
 * `reply`: 消息是对另一消息的答复的指示符，原始消息的唯一ID，`"grp1XUtEhjv6HND:123"`.
 * `sender`: 当代表另一用户发送消息时，服务器添加的发件人的用户ID，`"usr1XUtEhjv6HND"`.
 * `thread`: 消息是会话线程的一部分的指示符，线程中第一条消息的主题唯一ID， `":123"`; `thread` 用于标记消息的平面列表，与创建树相反。
 * `webrtc`: 表示消息所代表的视频呼叫状态的字符串。可能值：
   * `"started"`: 呼叫已启动并正在建立
   * `"accepted"`: 呼叫已被接受并建立
   * `"finished"`: 先前成功建立的呼叫已结束
   * `"missed"`: 呼叫方挂断或在建立之前超时
   * `"declined"`: 呼叫在建立之前被被呼叫方挂断
   * `"disconnected"`: 由于其他原因（例如，由于错误），服务器终止了呼叫
 * `webrtc-duration`: 表示建立后视频通话持续时间的数字（以毫秒为单位）。

特定于应用程序的字段应以`x-<application-name>-`开头。虽然服务器尚未强制执行此规则，但将来可能会开始执行此规则。

唯一的消息ID应尽可能形成为`<topic_name>:<seqId>` ，例如 `"grp1XUtEhjv6HND:123"`. 如果省略了主题，即`":123"`,则假定它是当前主题。

#### `{get}`

查询主题以获取元数据，如描述或订阅者列表，或查询消息历史记录。请求者必须[[订阅并附加](#sub) 到主题才能收到完整的响应。一些有限的`desc`和`sub`信息可以不附加。

```js
get: {
  id: "1a2b3", // string, 客户端提供的消息id，可选
  topic: "grp1XUtEhjv6HND", // string, 请求数据的主题名称
  what: "sub desc data del cred", // string, 要查询的参数的空格分隔列表；忽略未知值；必需的
            // {get what="desc"}的可选参数
  desc: {
    ims: "2015-10-06T18:07:30.038Z" // timestamp, 指定的时间戳之后至少更新了其中一个值时，才返回公共值和私有值，可选
  },

  // {get what="sub"}的可选参数
  sub: {
    ims: "2015-10-06T18:07:30.038Z", // timestamp, 时间戳，“如果自修改”-
              //仅当在指定的时间戳之后至少更新了其中一个值时，
              //才返回公共值和私有值，可选
    user: "usr2il9suCbuko", // string, 返回单个用户的结果，
                            //除'me'以外的任何主题，可选
    topic: "usr2il9suCbuko", // string, 返回单个主题的结果，
                          //仅限'me' 主题，可选
    limit: 20 // integer, 限制返回对象的数量
  },

  // {get what="data"}的可选参数
  data: {
    since: 123, // integer, 加载服务器发出ID大于或等于的消息
                //到此（包含/关闭），可选
    before: 321, // integer, 加载服务器issed序列ID小于此（独占/打开）的消息，可选
    limit: 20, // integer, 限制返回对象的数量，默认值：32，
                //可选的
  },

  // {get what="del"}的可选参数
  del: {
    since: 5, // integer, 加载服务器发出ID大于或等于的消息
                //到此（包含/关闭），可选
    before: 12, // integer, 加载删除事务ID小于this的已删除范围（独占/打开），可选
    limit: 25, // integer, 整数，限制返回对象的数量，默认值：32，
  }
}
```

* `{get what="desc"}`

查询主题描述。服务器以包含请求数据的`{meta}` 消息进行响应。有关详细信息，请参见`{meta}`。
如果指定了 `ims` 且数据尚未更新，则消息将跳过`trusted`, `public`和`private`字段。

在没有[附加](#sub) 主题的情况下，可获得的信息有限。

有关`private` 和`public`格式的注意事项，请参见[公共和私有字段](#public-and-private-fields)。

* `{get what="sub"}`

获取订阅者列表。服务器以包含订户列表的`{meta}`消息进行响应。有关详细信息，请参见`{meta}`。
对于`me` 主题，请求返回用户订阅的列表。如果指定了`ims`并且数据尚未更新，
响应`{ctrl}` "not modified"消息。

只返回用户自己的订阅，而不首先将[附加](#sub)到主题。

* `{get what="tags"}`

查询索引标记。服务器以包含字符串标记数组的`{meta}`消息进行响应。有关详细信息，请参阅`{meta}`和`fnd` 主题。
仅支持`me`和组主题。

* `{get what="data"}`

查询消息历史记录。服务器发送与查询的`data` 字段中提供的参数匹配的`{data}`消息。
没有提供数据消息的`id`字段，因为它是数据消息的常用字段。当所有`{data}`消息被发送时，发送`{ctrl}` 信息。

* `{get what="del"}`

查询邮件删除历史记录。服务器以包含已删除消息范围列表的`{meta}`消息进行响应。

* `{get what="cred"}`

查询[凭据](#credentail-validation)。服务器以包含凭据数组的`{meta}`消息进行响应。仅支持`me`主题。

#### `{set}`

更新主题元数据，删除消息或主题。通常期望请求者[订阅并附加](#sub)到主题。只有`desc.private`和请求者的`sub.mode`可以在不首先附加的情况下更新。

```js
set: {
  id: "1a2b3", // string, 客户端提供的消息id，可选
  topic: "grp1XUtEhjv6HND", // string, 要更新的主题名称，必填

  // 用于更新主题描述的可选负载
  desc: {
    defacs: { // 新建默认访问模式
      auth: "JRWP",  // 已验证用户的访问权限
      anon: "JRW" // 匿名用户的访问权限
    },
    trusted: { ... }, // 由系统管理分配的应用程序定义的负载
    public: { ... }, // 用于描述主题的应用程序定义的负载
    private: { ... } // 每个用户的专用应用程序定义的内容
  },

  // 用于更新订阅的可选负载
  sub: {
    user: "usr2il9suCbuko", // string, 受此请求影响的用户；
                            //默认值（空）表示当前用户
    mode: "JRWP" // 访问模式更改，给定 ('user'
                 // 已定义) or 或请求 ('user' 未定义)
  }, // object, what == "sub"的有效负载

  // 标记的可选更新（请参阅fnd主题描述）
  tags: [ // 字符串数组
    "email:alice@example.com", "tel:1234567890"
  ],

  cred: { // 可选更新凭据。
    meth: "email", // string, 验证方法，例如 "email", "tel", "recaptcha"等。
    val: "alice@example.com", // string, 要验证的凭据，如电子邮件或电话
    resp: "178307", // string, 验证响应，可选
    params: { ... } // 参数，特定于验证方法，可选
  }
}
```

#### `{del}`

删除邮件、订阅、主题和用户。

```js
del: {
  id: "1a2b3", // string, 客户端提供的消息id，可选
  topic: "grp1XUtEhjv6HND", // string, 主题受影响， "topic", "sub","msg"所需，
  what: "msg", // string, one of "topic", "sub", "msg", "user", "cred"之一；
               //要删除的内容-整个主题、订阅、部分或全部消息、用户、凭证；可选，默认值： "msg"
  hard: false, // boolean, 请求硬删除vs标记为已删除；
                //如果what="msg"，则删除所有用户与当前用户；
               //可选，默认值：false
  delseq: [{low: 123, hi: 125}, {low: 156}], // 要删除的消息ID范围数组，包括独占，即 [low, hi), 可选
  user: "usr2il9suCbuko" // string, 正在删除的用户 (what="user")或正在删除其订阅(what="sub"),，可选 (what="sub"), optional
  cred: { // credential to delete (仅限'me' 主题).
    meth: "email", // string, 验证方法，例如"email", "tel"等。
    val: "alice@example.com" // string, 正在删除凭据
  }
}
```

`what="msg"`

用户可以软删除`hard=false`（默认）或硬删除`hard=true`消息。软删除消息对请求用户隐藏，但不会从存储中删除。软删除邮件需要`R`权限。硬删除消息会从存储中删除消息内容(`head`, `content`) ，留下消息存根。它影响所有用户。硬删除邮件需要`D` 权限。通过在`delseq`参数中指定一个或多个消息ID范围，可以批量删除消息。每个删除操作都分配一个唯一的`delete ID`。最大的`delete ID`在`{meta}`消息的`clear` 中报告。

`what="sub"`

删除订阅将从主题订阅者中删除指定用户。它需要`A`权限。用户无法删除自己的订阅。应改用`{leave}`。如果订阅是软删除的（默认），则会标记为已删除，而不会从存储中实际删除记录。

`what="topic"`

删除主题将删除主题，包括所有订阅和所有消息。只有所有者才能删除主题。

`what="user"`

删除用户是一项非常繁重的操作。小心操作。

`what="cred"`

删除凭据。已验证的凭据和未尝试验证的凭据将被硬删除。验证尝试失败的凭据将被软删除，从而防止同一用户重复使用。


#### `{note}`

客户端生成的临时通知，用于转发到当前附加到主题的其他客户端，例如键入通知或交货收据。消息是"点火并忘记"：不存储到磁盘本身，也不被服务器确认。被视为无效的消息将自动丢弃。
`{note.recv}` 和`{note.read}` 确实会改变服务器上的持久状态。该值被存储并报告回`{meta.sub}`消息的相应字段中。

```js
note: {
  topic: "grp1XUtEhjv6HND", // string, 要通知的主题，必填
  what: "kp", // string, one of "kp" (按键), "read" (读取通知),
              // "recv" (收到的通知), "data" (表单响应或其他结构化数据);
              // 任何其他字符串都会导致消息被忽略，这是必需的。
  seq: 123,   // integer, I要确认的消息的ID，需要
              // 'recv' 和 'read'.
  unread: 10, // integer, 客户端报告的未读邮件总数，可选。
  data: {     // object, 'data'所需的负载。
    ...
  }
}
```

目前已确认以下行动:
 * kp: 按键，即键入通知。客户端应该使用它来指示用户正在编写新消息。
 * recv: 客户端软件接收到`{data}` 消息，但用户可能尚未看到。
 * read: 用户看到 `{data}` 消息。它也暗示 `recv`.
 * data: 结构化数据的通用包，通常是表单响应。

`read`和`recv`通知可以可选地包括`unread`值，该值是此客户端确定的未读邮件总数。每个用户的`unread` 计数由服务器维护：当新的`{data}`消息发送给用户并重置为`{note unread=...}`邮件报告的值时，该计数将递增。服务器从不递减`unread` 值。该值包含在推送通知中，将显示在iOS上的徽章上：
<p align="center">
  <img src="./ios-pill-128.png" alt="Tinode iOS icon with a pill counter" width=64 height=64 />
</p>


### 服务器到客户端消息

响应特定请求生成的会话消息包含一个`id`字段，该字段等于
原始消息。服务器不解释`id`。

大多数服务器到客户端消息都有一个`ts`字段，它是服务器生成消息时的时间戳。

#### `{data}`

主题中发布的内容。这些消息是数据库中保留的唯一消息`｛data｝`消息是
向具有`R` 权限的所有主题订户广播。

```js
data: {
  topic: "grp1XUtEhjv6HND", // string, 分发此消息的主题，
                          //始终存在
  from: "usr2il9suCbuko", // string, 发布消息的用户的id；如果消息是
                          //由服务器生成
  head: { key: "value", ... }, // 字符串键值对集，已传递
                          //未从{pub}更改，可选
  ts: "2015-10-06T18:07:30.038Z", // string, 时间戳
  seq: 123, // integer, 服务器发出的顺序ID
  content: { ... } // object, 应用程序定义的内容与发布的内容完全相同
              // 由用户在{pub} 消息中by 
}
```

数据消息有一个`seq`字段，其中包含服务器生成的顺序数字ID。保证ID在主题中是唯一的。ID从1开始，并随着主题收到的每一条成功的[`{pub}`]（#pub）消息而依次递增。

有关“内容”格式的注意事项，请参见[内容格式](#format-of-content)。

有关`head` 字段的可能值，请参见[`{pub}`](#pub)消息。

#### `{ctrl}`

指示错误或成功条件的一般响应。消息将发送到发起会话。

```js
ctrl: {
  id: "1a2b3", // string, 客户端提供的消息id，可选
  topic: "grp1XUtEhjv6HND", // string, 主题名称，如果这是主题上下文中的响应，则为可选
  code: 200, // integer, 指示请求成功或失败的代码，遵循HTTP状态代码模型，始终存在
  text: "OK", // string, 包含有关结果的更多详细信息的文本，始终存在
  params: { ... }, // object, 通用响应参数，上下文相关，
                  //可选的
  ts: "2015-10-06T18:07:30.038Z", // string, 时间戳
}
```

#### `{meta}`

响应于“｛get｝”、“｛set｝”或“｛sub｝”消息向发起会话发送的有关主题元数据或订阅者的信息。

```js
meta: {
  id: "1a2b3", // string, 客户端提供的消息id，可选
  topic: "grp1XUtEhjv6HND", 主题名称，如果这是
              //主题上下文，可选
  ts: "2015-10-06T18:07:30.038Z", // string, 时间戳
  desc: {
    created: "2015-10-24T10:26:09.716Z",
    updated: "2015-10-24T10:26:09.716Z",
    status: "ok", // 账户状态；仅包含在`me`主题中，并且仅当请求由根身份验证会话发送时。
    defacs: { // topic的默认访问权限；仅当当前
              //用户具有'S'权限
      auth: "JRWP", // 已验证用户的默认访问权限
      anon: "N" // 匿名用户的默认访问权限
    },
    acs: {  // 用户的实际访问权限
      want: "JRWP", // string, 请求的访问权限
      given: "JRWP", // string, 授予访问权限
    mode: "JRWP" // string, want和给定的组合
    },
    seq: 123, // integer, 服务器发出的最后一条｛data｝消息的id
    read: 112, // integer, 用户通过｛note｝消息声明的消息ID
               //阅读，可选
    recv: 115, // integer, 类似'read'，但已接收，可选
    clear: 12, // integer, 如果某些邮件被删除，则为已删除邮件的最大ID，可选
    trusted: { ... }, // 由系统管理分配的应用程序定义的负载
    public: { ... }, // 所有主题订阅者都可以使用的应用程序定义的数据
    private: { ...} // 仅对当前用户可用的应用程序定义的数据
  }, // object, 主题描述，可选
  sub:  [ // 对象数组、主题订阅者或用户订阅，可选
    {
      user: "usr2il9suCbuko", // 此订阅描述的用户ID，在查询'me'时不存在。
      updated: "2015-10-24T10:26:09.716Z", // 订阅中最后一次更改的时间戳，仅适用于请求者自己的订阅
      touched: "2017-11-02T09:13:55.530Z", // 主题中最后一条消息的时间戳（也可能包括将来的其他事件，例如新订户
      acs: {  // 用户的访问权限
        want: "JRWP", // string, 请求的访问权限，当请求者是主题的管理员或所有者时，为用户自己的订阅提供
        given: "JRWP", // string, 授予访问权限，可选，与'want'完全相同 
        mode: "JRWP" // string, want和given的组合
      },
      read: 112, // integer, 用户通过｛note｝消息声明的消息ID阅读，可选。
      recv: 315, // integer, 类似'read'，但已接收，可选。
      clear: 12, // integer, 如果某些消息被删除，则为已删除消息的最大ID，可选。
      trusted: { ... }, // 由系统管理分配的应用程序定义的负载
      public: { ... }, // 应用程序定义的用户的'public'对象，在查询P2P主题时不存在。 
      private: { ... } // 应用程序定义的用户的'private'对象。
      online: true, // boolean, 用户的当前在线状态；如果这是一个组或一个p2p主题，则表示用户在主题中的在线状态，
                    //即用户是否已连接并正在收听消息；如果这是对'me'查询的响应，它会告诉主题是否在线；
                    //如果另一方在线，则p2p被认为是在线的，不一定依附于主题；
                    //如果组主题具有至少一个活动订户，则该组主题被认为是在线的。

      // 以下字段仅在查询'me'主题时出现

      topic: "grp1XUtEhjv6HND", // string, 此订阅描述的主题
      seq: 321, // integer, 服务器发出的最后一条{data}消息的

      // 以下字段仅在查询'me'主题且所描述的主题是P2P主题时出现
      seen: { // object, 如果这是P2P主题，请提供该节点上次联机的时间信息
        when: "2015-10-24T10:26:09.716Z", // timestamp
        ua: "Tinode/1.0 (Android 5.1)" // string, 节点客户端的用户代理
      }
    },
    ...
  ],
  tags: [ // 主题或用户（如果是"me"主题）索引的标记数组
    "email:alice@example.com", "tel:+1234567890"
  ],
  cred: [ // 用户凭据数组
    {
      meth: "email", // string, 验证方法
      val: "alice@example.com", // string, 凭据值
      done: true     // 验证状态
    },
    ...
  ],
  del: {
    clear: 3, // 最新适用的'delete'事务的ID
    delseq: [{low: 15}, {low: 22, hi: 28}, ...], // 已删除邮件的ID范围
  }
}
```

#### `{pres}`

Tinode 使用 `{pres}` m消息通知客户端重要事件。一份单独的 [文档](https://docs.google.com/spreadsheets/d/e/2PACX-1vStUDHb7DPrD8tF5eANLu4YIjRkqta8KOhLvcj2precsjqR40eDHvJnnuuS3bw-NcWsP1QKc7GSTYuX/pubhtml?gid=1959642482&single=true) 解释了所有可能的用例。

```js
pres: {
  topic: "me", // string, 接收通知的主题，始终存在
  src: "grp1XUtEhjv6HND", // string, 受更改影响的主题或用户，始终存在
  what: "on", // string, 更改的内容，始终存在
  seq: 123, // integer, "what" 是 "msg"，服务器发出的消息ID，可选
  clear: 15, // integer, "what" 是 "del"，更新删除事务ID。
  delseq: [{low: 123}, {low: 126, hi: 136}], // 范围数组, "what" 是 "del",
             // 已删除邮件的ID范围，可选
  ua: "Tinode/1.0 (Android 2.2)", // string, 如果"what"是"on" 或"ua"，则标识客户端软件的用户代理字符串，可选
  act: "usr2il9suCbuko",  // string, 执行操作的用户，可选
  tgt: "usrRkDVe0PYDOo",  // string, 受操作影响的用户，可选
  acs: {want: "+AS-D", given: "+S"} // object, 更改访问模式，"what"是"acs"，可选
}
```

`{pres}` 消息纯粹是暂时的：它们不会被存储，如果目的地暂时不可用，也不会尝试稍后传递它们。

`{pres}` 消息中不存在时间戳。


#### `{info}`

转发的客户端生成的通知`{note}`。服务器保证消息符合此规范，并且`topic` 和`from`字段的内容正确。其他内容是从`{note}`消息中逐字复制的，如果发件人愿意，可能会不正确或误导。

```js
info: {
  topic: "grp1XUtEhjv6HND", // string, 主题受影响，始终存在
  from: "usr2il9suCbuko", // string, 发布消息的用户的id，始终存在
  what: "read", // string, "kp", "recv", "read", "data", 之一，请参见客户端 {note}, 始终存在
  seq: 123, // integer, 客户端已确认的消息ID，保证0 < read <= recv <= {ctrl.params.seq}；存在以供recv 和 read
}
```
