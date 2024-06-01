

```text

    process <--------> client <--------> forwarder <--------> proxy-server <--------> server

```

还有builtin 作用到哪？作用到proxy-server

方案一：
    转发器可以复用当前的proxy-server， forwarder是Server，但是Sender需要专门实现，如果作为Client, 可能端口有限（ports不能重用）。

    问题： builtin无法解决，总不能将两个user-tcp 串联起来

方案二：
    改协议, peer携带两个地址。。。

    不太理智, 而且client实际上无法知道proxy-server地址。


√ 方案三：
    单独写网络层转发，forward 需要进行智能路由，限流等操作