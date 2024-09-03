# SeeFlow
> Currently, this project has been merged into https://gitee.com/stleox/cilium-v1.16/seeflow 

SeeFlow provides insight into Cilium's flow, which is observability data on
Kubernetes platform.

Cilium version 1.16 refers to this [fork](https://gitee.com/stleox/cilium-v1.16).

## Usage

### Observe

用于单次运行模式，例如：

```shell
# 采集过往 10 秒以内的流量。
./seeflow --debug observe --since 10s
```

### Serve

用于长期运行模式，例如：

```shell
# 启动采集服务。
./seeflow serve
```
