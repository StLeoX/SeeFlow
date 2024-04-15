# SeeFlow

SeeFlow provides insight into Cilium's flow,which is observability data on
Kubernetes platform.

## Usage

### Observe

用于单次运行模式，例如：

```shell
./seeflow --debug observe --since 10s
```

采集过往 10 秒以内的流量。

### Serve

用于长期运行模式，例如：

```shell
./seeflow serve
```

启动服务。