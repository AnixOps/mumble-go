# mumble-go 项目路线图

## 项目定位

mumble-go 是用 Go 语言实现的 Mumble 语音聊天协议客户端库和 SDK，支持：

- Mumble 服务器连接、用户管理、频道操作
- Opus 音频编解码（TCP Tunnel 和原生 UDP 两种模式）
- 音频流接收与播放
- Streaming Sender SDK - 商业级音频流发送，支持多源混音、VIT、Jitter Buffer、断线重连

**对标版本**：Mumble 1.4.x 兼容，优先与 [Mumble 1.6.x](https://github.com/mumble-voip/mumble/tree/1.6.x) 对齐

---

## 已实现功能

| 模块 | 能力 |
|------|------|
| client | 连接、认证、频道管理、消息收发 |
| protocol | Mumble 协议完整实现（proto Marshal/Unmarshal） |
| audio | Opus 编解码、音频输入输出、UDP Manager、Crypto |
| state | 服务器状态同步（用户/频道 Store） |
| identity | TLS 证书身份系统 |
| sdk | 高层 API + Streaming Sender SDK |
| sdk/stream | 多源混音、Jitter Buffer、VAD、重连管理器、元数据 |

---

## v1.0: Streaming Sender 完善

### 目标
Streaming Sender 是核心商业功能，需确保生产可用。

### 任务

- [ ] **Opus 编码器直接集成**
  - 当前依赖 `hraban/opus` 通过 `audio.Output` 间接编码
  - 暴露独立 `OpusEncoder` 供 `StreamSender` 直接使用
  - 降低耦合，提高性能

- [ ] **Jitter Buffer 实际使用**
  - `JitterBuffer` 已实现但 `sender.go` 中尚未接入主循环
  - 接入主循环，实现实际抖动平滑

- [ ] **ReconnectManager 深度集成**
  - 重连时音频缓冲重放
  - client 重连回调自动触发
  - 参考 `GetReconnectManager()` 接口设计

- [ ] **VAD 静音跳过发送**
  - 目前 VAD 只通知不断言
  - 增加 `SkipSilentFrames` 配置
  - 网络带宽优化

- [ ] **Metadata 完整实现**
  - `UserState.Comment` JSON 序列化
  - 服务器同步机制

---

## v1.1: 音频接收与播放

### 目标
完成双向音频通信，支持接收和播放远程音频。

### 任务

- [ ] **音频播放引擎**
  - 集成音频输出（speaker）
  - 支持多用户音频混合播放

- [ ] **音频路由**
  - 接收到的音频可路由到不同输出设备
  - 按用户/频道过滤音频

- [ ] **Jitter Buffer 接收端**
  - 接收方的 jitter buffer 处理网络抖动
  - 平滑播放，减少卡顿

---

## v1.2: 协议扩展

### 目标
扩展 Mumble 协议支持，实现高级功能。

### 任务

- [ ] **Whisper（私语）**
  - 定向发送音频到特定用户/频道
  - 权限检查

- [ ] **Channel Tree 监听**
  - 订阅特定频道而非全局音频
  - 减少不必要的音频处理

- [ ] **Opus 头字节携带 session**
  - UDP 模式下接收方能从 opus 头解析 session
  - 减少信令开销

- [ ] **ACL/Permission 权限管理**
  - 细粒度权限控制
  - 角色和组支持

- [ ] **BanList 用户封禁管理**
  - 封禁用户列表操作

---

## v1.3: SDK 层面完善

### 目标
提升开发者体验，完善生态。

### 任务

- [ ] **go.mod 清理**
  - `hraban/opus` 声明为 direct dependency

- [ ] **文档**
  - godoc 完整注释和示例
  - API 使用指南

- [ ] **单元测试覆盖**
  - jitter、vad、mixer、reconnect 核心组件测试
  - 基准测试

- [ ] **集成测试**
  - 连接真实 Mumble 服务器的压力测试
  - CI/CD 自动化

---

## v1.4: 工具与应用

### 目标
提供可用的示例程序和工具。

### 任务

- [ ] **stream_demo 完善**
  - 演示 VAD、重连、多源混音
  - 完整的使用示例

- [ ] **mumble-sdkd 完善**
  - 作为守护进程长期运行
  - 支持配置和远程控制

- [ ] **CLI 工具**
  - 简单命令行客户端用于调试
  - 基本交互功能

---

## v1.5: 性能和可靠性

### 目标
生产环境级别的稳定性和性能。

### 任务

- [ ] **连接池**
  - 支持多个并发客户端
  - 资源复用

- [ ] **TLS 持久化**
  - 减少握手延迟
  - 连接复用

- [ ] **优雅关闭**
  - 所有 goroutine 正确退出
  - 资源清理

- [ ] **Metrics/Observability**
  - 连接时长统计
  - 音频帧率监控
  - 延迟统计

---

## 使用场景

### 音乐 Bot

```go
sender, _ := stream.NewStreamSender(client, cfg)
sender.AddSource("music", ffmpegSrc, 1.0)
sender.AddSource("sfx", sfxSrc, 0.3)
sender.SetMetadata(&stream.StreamMetadata{Title: "Playing..."})
sender.Start(ctx)
```

### 语音网关
- 桥接其他协议（Discord、Teamspeak）到 Mumble

### 播客推流
- 持续流式传输预录制内容到 Mumble 频道

---

## 技术栈

- Go 1.21+
- github.com/hraban/opus - Opus 编解码
- TLS 1.3 加密
- OCB-AES-128 语音加密
- 协议版本：Mumble 1.4.x 兼容

---

## 里程碑

| 版本 | 目标 | 关键交付物 |
|------|------|-----------|
| v0.1 | 协议基础 | 连接、认证、状态同步 |
| v0.2 | 音频基础 | Opus 编解码、UDP 传输 |
| v0.3 | Streaming Sender | 多源混音、VAD、重连、Jitter Buffer |
| **v1.0** | **生产就绪** | **Streaming Sender 完善** |
| v1.1 | 双向音频 | 音频接收与播放 |
| v1.2 | 协议扩展 | Whisper、ACL、Channel 监听 |
| v1.3+ | 生态完善 | 文档、测试、工具 |
