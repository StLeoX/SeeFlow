filter 是很强的案例驱动的，在此迭代 filter 的设计。

**filter rules**
一些普遍的编写规则：
`-4` 目前是 IPv4，不过负载一般不涉及到 v6。
`--all` 指定范围，还有 `--first`、`--last`、`--since` 等方式
`-o json` 指定输出格式。

**l34_flow**
l34_flow 的 type 是 trace。
目前排除掉 dns 相关流量，因为 dns 是 payload 无关的。

```shell
hubble observe -t trace --namespace deepflow-spring-demo --not --label "k8s:k8s-app=kube-dns" 
```

`--namespace` 是一个“或”的逻辑，联合了 `--from-namespace` 和 `--to-namespace` 这两种情况。


**l7_flow**
l34_flow 的 type 是 l7。
