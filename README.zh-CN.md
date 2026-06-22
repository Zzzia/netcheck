# netcheck

[English](README.md)

`netcheck` 是给 vibe coding 开发者使用的全平台网络质量面板，用来判断 Codex 变慢到底是模型、客户端，还是底层网络链路造成的。

网络波动会直接放大 Codex 的交互耗时。把 Codex 开到 fast，不如先让网络质量变好；同样一次 vibe coding 任务，在网络差和网络正常的情况下，整体耗时可能相差一倍。`netcheck` 一边监控本机、国内、国外链路波动，一边直接读取 Codex 请求结果，让网页面板给出明确的网络质量评价，而不是只能靠“感觉慢”来判断。

<table>
  <tr>
    <td width="50%"><img src="img/cn-main.webp" width="100%" alt="netcheck 中文统计概览"></td>
    <td width="50%"><img src="img/cn-detail.webp" width="100%" alt="netcheck 中文详细指标和 Codex 时间轴"></td>
  </tr>
  <tr>
    <td align="center">统计概览</td>
    <td align="center">详细指标和 Codex 时间轴</td>
  </tr>
</table>

## 快速开始

构建并启动：

```bash
make build
./netcheck
```

打开终端输出的本地 URL。默认情况下，网页面板监听 `0.0.0.0:8765`；如果端口被占用，`netcheck` 会自动切到下一个可用端口，并打印实际访问地址。

## 面板内容

- 网关 RTT、抖动和丢包率。
- 国内、国外链路的延迟、下载速度和失败率。
- 当前时间范围内的异常持续时间和最近网络事件。
- Codex 断流重试、受影响 turn、网络错误候选和最大重试深度。
- 对齐网络探针和 Codex 请求时间轴，帮助区分“模型本身慢”和“网络让同一段 Codex 工作流变慢”。

## License

暂未提供。
